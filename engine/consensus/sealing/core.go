package sealing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gammazero/workerpool"
	"github.com/onflow/crypto/hash"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	otelTrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/consensus"
	"github.com/onflow/flow-go/engine/consensus/approvals"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state/fork"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// Core is an implementation of SealingCore interface
// This struct is responsible for:
//   - collecting approvals for execution results
//   - processing multiple incorporated results
//   - pre-validating approvals (if they are outdated or non-verifiable)
//   - pruning already processed collectorTree
type Core struct {
	workerPool                 *workerpool.WorkerPool             // worker pool used by collectors
	log                        zerolog.Logger                     // used to log relevant actions with context
	collectorTree              *approvals.AssignmentCollectorTree // levelled forest for assignment collectors
	approvalsCache             *approvals.LruCache                // in-memory cache of approvals that weren't verified
	counterLastSealedHeight    counters.StrictMonotonicCounter    // monotonic counter for last sealed block height
	counterLastFinalizedHeight counters.StrictMonotonicCounter    // monotonic counter for last finalized block height
	headers                    storage.Headers                    // used to access block headers in storage
	state                      protocol.State                     // used to access protocol state
	seals                      storage.Seals                      // used to get last sealed block
	sealsMempool               mempool.IncorporatedResultSeals    // used by tracker.SealingObservation to log info
	requestTracker             *approvals.RequestTracker          // used to keep track of number of approval requests, and blackout periods, by chunk
	metrics                    module.ConsensusMetrics            // used to track consensus metrics
	sealingTracker             consensus.SealingTracker           // logic-aware component for tracking sealing progress.
	tracer                     module.Tracer                      // used to trace execution
	sealingConfigsGetter       module.SealingConfigsGetter        // used to access configs for sealing conditions
	reporter                   *gatedSealingObservationReporter   // used to avoid excess resource usage by sealing observation completions
}

func NewCore(
	log zerolog.Logger,
	workerPool *workerpool.WorkerPool,
	tracer module.Tracer,
	conMetrics module.ConsensusMetrics,
	sealingTracker consensus.SealingTracker,
	headers storage.Headers,
	state protocol.State,
	sealsDB storage.Seals,
	assigner module.ChunkAssigner,
	signatureHasher hash.Hasher,
	sealsMempool mempool.IncorporatedResultSeals,
	approvalConduit network.Conduit,
	sealingConfigsGetter module.SealingConfigsGetter,
) (*Core, error) {
	lastSealed, err := state.Sealed().Head()
	if err != nil {
		return nil, fmt.Errorf("could not retrieve last sealed block: %w", err)
	}

	core := &Core{
		log:                        log.With().Str("engine", "sealing.Core").Logger(),
		workerPool:                 workerPool,
		tracer:                     tracer,
		metrics:                    conMetrics,
		sealingTracker:             sealingTracker,
		approvalsCache:             approvals.NewApprovalsLRUCache(1000),
		counterLastSealedHeight:    counters.NewMonotonicCounter(lastSealed.Height),
		counterLastFinalizedHeight: counters.NewMonotonicCounter(lastSealed.Height),
		headers:                    headers,
		state:                      state,
		seals:                      sealsDB,
		sealsMempool:               sealsMempool,
		requestTracker:             approvals.NewRequestTracker(headers, 10, 30),
		sealingConfigsGetter:       sealingConfigsGetter,
		reporter:                   newGatedSealingObservationReporter(),
	}

	factoryMethod := func(result *flow.ExecutionResult) (approvals.AssignmentCollector, error) {
		requiredApprovalsForSealConstruction := sealingConfigsGetter.RequireApprovalsForSealConstructionDynamicValue()
		base, err := approvals.NewAssignmentCollectorBase(core.log, core.workerPool, result, core.state, core.headers,
			assigner, sealsMempool, signatureHasher,
			approvalConduit, core.requestTracker, requiredApprovalsForSealConstruction)
		if err != nil {
			return nil, fmt.Errorf("could not create base collector: %w", err)
		}
		return approvals.NewAssignmentCollectorStateMachine(base), nil
	}

	core.collectorTree = approvals.NewAssignmentCollectorTree(lastSealed, headers, factoryMethod)

	return core, nil
}

// RepopulateAssignmentCollectorTree restores latest state of assignment collector tree based on local chain state information.
// Repopulating is split into two parts:
// 1) traverse forward all finalized blocks starting from last sealed block till we reach last finalized block . (lastSealedHeight, lastFinalizedHeight]
// 2) traverse forward all unfinalized(pending) blocks starting from last finalized block.
// For each block that is being traversed we will collect execution results and process them using sealing.Core.
func (c *Core) RepopulateAssignmentCollectorTree(payloads storage.Payloads) error {
	finalizedSnapshot := c.state.Final()
	finalized, err := finalizedSnapshot.Head()
	if err != nil {
		return fmt.Errorf("could not retrieve finalized block: %w", err)
	}
	finalizedID := finalized.ID()

	// Get the latest sealed block on this fork, ie the highest block for which
	// there is a seal in this fork.
	latestSeal, err := c.seals.HighestInFork(finalizedID)
	if err != nil {
		return fmt.Errorf("could not retrieve parent seal (%x): %w", finalizedID, err)
	}

	latestSealedBlockID := latestSeal.BlockID
	latestSealedBlock, err := c.headers.ByBlockID(latestSealedBlockID)
	if err != nil {
		return fmt.Errorf("could not retrieve latest sealed block (%x): %w", latestSealedBlockID, err)
	}

	// Get the root block of our local state - we allow references to unknown
	// blocks below the root height
	rootHeader := c.state.Params().FinalizedRoot()

	// Determine the list of unknown blocks referenced within the sealing segment
	// if we are initializing with a latest sealed block below the root height
	outdatedBlockIDs, err := c.getOutdatedBlockIDsFromRootSealingSegment(rootHeader)
	if err != nil {
		return fmt.Errorf("could not get outdated block IDs from root segment: %w", err)
	}

	blocksProcessed := uint64(0)
	totalBlocks := finalized.Height - latestSealedBlock.Height

	// resultProcessor adds _all known_ results for the given block to the assignment collector tree
	resultProcessor := func(header *flow.Header) error {
		blockID := header.ID()
		payload, err := payloads.ByBlockID(blockID)
		if err != nil {
			return fmt.Errorf("could not retrieve index for block (%x): %w", blockID, err)
		}

		for _, result := range payload.Results {
			// skip results referencing blocks before the root sealing segment
			_, isOutdated := outdatedBlockIDs[result.BlockID]
			if isOutdated {
				c.log.Debug().
					Hex("container_block_id", logging.ID(blockID)).
					Hex("result_id", logging.ID(result.ID())).
					Hex("executed_block_id", logging.ID(result.BlockID)).
					Msg("skipping outdated block referenced in root sealing segment")
				continue
			}
			incorporatedResult, err := flow.NewIncorporatedResult(flow.UntrustedIncorporatedResult{
				IncorporatedBlockID: blockID,
				Result:              result,
			})
			if err != nil {
				return fmt.Errorf("could not create incorporated result for block (%x): %w", blockID, err)
			}
			err = c.ProcessIncorporatedResult(incorporatedResult)
			if err != nil {
				return fmt.Errorf("could not process incorporated result from block %s: %w", blockID, err)
			}
		}

		blocksProcessed++
		if (blocksProcessed%20) == 0 || blocksProcessed >= totalBlocks {
			c.log.Debug().Msgf("%d/%d have been loaded to collector tree", blocksProcessed, totalBlocks)
		}

		return nil
	}

	c.log.Info().Msgf("reloading assignments from %d finalized, unsealed blocks into collector tree", totalBlocks)

	// traverse chain forward to collect all execution results that were incorporated in this fork
	// we start with processing the direct child of the last finalized block and end with the last finalized block
	err = fork.TraverseForward(c.headers, finalizedID, resultProcessor, fork.ExcludingBlock(latestSealedBlockID))
	if err != nil {
		return fmt.Errorf("internal error while traversing fork: %w", err)
	}

	// at this point we have processed all results in range (lastSealedBlock, lastFinalizedBlock].
	// Now, we add all known results for any valid block that descends from the latest finalized block:
	validPending, err := finalizedSnapshot.Descendants()
	if err != nil {
		return fmt.Errorf("could not retrieve valid pending blocks from finalized snapshot: %w", err)
	}

	blocksProcessed = 0
	totalBlocks = uint64(len(validPending))

	c.log.Info().Msgf("reloading assignments from %d unfinalized blocks into collector tree", len(validPending))

	// We use AssignmentCollectorTree for collecting approvals for each incorporated result.
	// In order to verify the received approvals, the verifier assignment for each incorporated result
	// needs to be known.
	// The verifier assignment is random, its Source of Randomness (SoR) is only available if a valid
	// child block exists.
	// In other words, the parent of a valid block must have the SoR available. Therefore, we traverse
	// through valid pending blocks which already have a valid child, and load each result in those block
	// into the AssignmentCollectorTree.
	for _, blockID := range validPending {
		block, err := c.headers.ByBlockID(blockID)
		if err != nil {
			return fmt.Errorf("could not retrieve header for unfinalized block %x: %w", blockID, err)
		}

		parent, err := c.headers.ByBlockID(block.ParentID)
		if err != nil {
			return fmt.Errorf("could not retrieve header for unfinalized block %x: %w", block.ParentID, err)
		}

		err = resultProcessor(parent)
		if err != nil {
			return fmt.Errorf("failed to process results for unfinalized block %x at height %d: %w", blockID, block.Height, err)
		}
	}

	return nil
}

// processIncorporatedResult implements business logic for processing single incorporated result
// Returns:
// * engine.InvalidInputError - incorporated result is invalid
// * engine.UnverifiableInputError - result is unverifiable since referenced block cannot be found
// * engine.OutdatedInputError - result is outdated for instance block was already sealed
// * exception in case of any other error, usually this is not expected
// * nil - successfully processed incorporated result
func (c *Core) processIncorporatedResult(incRes *flow.IncorporatedResult) error {
	err := c.checkBlockOutdated(incRes.Result.BlockID)
	if err != nil {
		return fmt.Errorf("won't process outdated or unverifiable execution incRes %s: %w", incRes.Result.BlockID, err)
	}
	incorporatedBlock, err := c.headers.ByBlockID(incRes.IncorporatedBlockID)
	if err != nil {
		return fmt.Errorf("could not get block height for incorporated block %s: %w",
			incRes.IncorporatedBlockID, err)
	}
	incorporatedAtHeight := incorporatedBlock.Height

	// For incorporating blocks at heights that are already finalized, we check that the incorporating block
	// is on the finalized fork. Otherwise, the incorporating block is orphaned, and we can drop the result.
	if incorporatedAtHeight <= c.counterLastFinalizedHeight.Value() {
		finalizedID, err := c.headers.BlockIDByHeight(incorporatedAtHeight)
		if err != nil {
			return fmt.Errorf("could not retrieve finalized block at height %d: %w", incorporatedAtHeight, err)
		}
		if finalizedID != incRes.IncorporatedBlockID {
			// it means that we got incorporated incRes for a block which doesn't extend our chain
			// and should be discarded from future processing
			return engine.NewOutdatedInputErrorf("won't process incorporated incRes from orphan block %s", incRes.IncorporatedBlockID)
		}
	}

	// Get (or create) assignment collector for the respective result (atomic operation) and
	// add the assignment for the incorporated result to it. (No-op if assignment already known).
	// Here, we just add assignment collectors to the tree. Cleanup of orphaned and sealed assignments
	// IRs whenever new finalized block is processed
	lazyCollector, err := c.collectorTree.GetOrCreateCollector(incRes.Result)
	if err != nil {
		return fmt.Errorf("cannot create collector: %w", err)
	}
	err = lazyCollector.Collector.ProcessIncorporatedResult(incRes)
	if err != nil {
		return fmt.Errorf("could not process incorporated incRes: %w", err)
	}

	// process pending approvals only if it's a new collector
	// pending approvals are those we haven't received its incRes yet,
	// once we received a incRes and created a new collector, we find the pending
	// approvals for this incRes, and process them
	// newIncorporatedResult should be true only for one goroutine even if multiple access this code at the same
	// time, ensuring that processing of pending approvals happens once for particular assignment
	if lazyCollector.Created {
		err = c.processPendingApprovals(lazyCollector.Collector)
		if err != nil {
			return fmt.Errorf("could not process cached approvals:  %w", err)
		}
	}

	return nil
}

// ProcessIncorporatedResult processes incorporated result in blocking way. Concurrency safe.
// Returns:
// * exception in case of unexpected error
// * nil - successfully processed incorporated result
func (c *Core) ProcessIncorporatedResult(result *flow.IncorporatedResult) error {

	span, _ := c.tracer.StartBlockSpan(context.Background(), result.Result.BlockID, trace.CONSealingProcessIncorporatedResult)
	defer span.End()

	err := c.processIncorporatedResult(result)
	// We expect only engine.OutdatedInputError. If we encounter UnverifiableInputError or InvalidInputError, we
	// have a serious problem, because these results are coming from the node's local HotStuff, which is trusted.
	if engine.IsOutdatedInputError(err) {
		c.log.Debug().Err(err).Msgf("dropping outdated incorporated result %v", result.ID())
		return nil
	}

	return err
}

// checkBlockOutdated performs a sanity check if block is outdated
// Returns:
// * engine.UnverifiableInputError - sentinel error in case we haven't discovered requested blockID
// * engine.OutdatedInputError - sentinel error in case block is outdated
// * exception in case of unknown internal error
// * nil - block isn't sealed
func (c *Core) checkBlockOutdated(blockID flow.Identifier) error {
	block, err := c.headers.ByBlockID(blockID)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("failed to retrieve header for block %x: %w", blockID, err)
		}
		return engine.NewUnverifiableInputError("no header for block: %v", blockID)
	}

	// it's important to use atomic operation to make sure that we have correct ordering
	lastSealedHeight := c.counterLastSealedHeight.Value()
	// drop approval, if it is for block whose height is lower or equal to already sealed height
	if lastSealedHeight >= block.Height {
		return engine.NewOutdatedInputErrorf("requested processing for already sealed block height")
	}

	return nil
}

// ProcessApproval processes approval in blocking way. Concurrency safe.
// Returns:
// * exception in case of unexpected error
// * nil - successfully processed result approval
func (c *Core) ProcessApproval(approval *flow.ResultApproval) error {
	c.log.Debug().
		Str("result_id", approval.Body.ExecutionResultID.String()).
		Str("verifier_id", approval.Body.ApproverID.String()).
		Msg("processing result approval")

	span, _ := c.tracer.StartBlockSpan(context.Background(), approval.Body.BlockID, trace.CONSealingProcessApproval)
	span.SetAttributes(
		attribute.String("approverId", approval.Body.ApproverID.String()),
		attribute.Int64("chunkIndex", int64(approval.Body.ChunkIndex)),
	)
	defer span.End()

	startTime := time.Now()
	err := c.processApproval(approval)
	c.metrics.OnApprovalProcessingDuration(time.Since(startTime))

	if err != nil {
		if engine.IsOutdatedInputError(err) {
			return nil // potentially delayed input
		}

		lg := c.log.With().
			Err(err).
			Str("approver_id", approval.Body.ApproverID.String()).
			Str("executed_block_id", approval.Body.BlockID.String()).
			Str("result_id", approval.Body.ExecutionResultID.String()).
			Str("approval_id", approval.ID().String()).
			Logger()
		if engine.IsUnverifiableInputError(err) {
			lg.Warn().Msg("received approval for unknown block (this node is potentially behind)")
			return nil
		}
		if engine.IsInvalidInputError(err) {
			lg.Error().Msg("received invalid approval")
			return nil
		}
		lg.Error().Msg("unexpected error processing result approval")

		return fmt.Errorf("internal error processing result approval %x: %w", approval.ID(), err)
	}

	return nil
}

// processApproval implements business logic for processing single approval
// Returns:
// * engine.InvalidInputError - result approval is invalid
// * engine.UnverifiableInputError - result approval is unverifiable since referenced block cannot be found
// * engine.OutdatedInputError - result approval is outdated for instance block was already sealed
// * exception in case of any other error, usually this is not expected
// * nil - successfully processed result approval
func (c *Core) processApproval(approval *flow.ResultApproval) error {
	err := c.checkBlockOutdated(approval.Body.BlockID)
	if err != nil {
		return fmt.Errorf("won't process approval for oudated block (%x): %w", approval.Body.BlockID, err)
	}

	if collector := c.collectorTree.GetCollector(approval.Body.ExecutionResultID); collector != nil {
		// if there is a collector it means that we have received execution result and we are ready
		// to process approvals
		err = collector.ProcessApproval(approval)
		if err != nil {
			return fmt.Errorf("could not process assignment: %w", err)
		}
	} else {
		c.log.Debug().
			Str("result_id", approval.Body.ExecutionResultID.String()).
			Msg("haven't yet received execution result, caching for later")

		// in case we haven't received execution result, cache it and process later.
		c.approvalsCache.Put(approval)
	}

	return nil
}

// checkEmergencySealing triggers the AssignmentCollectors to check whether satisfy the conditions to
// generate an emergency seal. To limit performance impact of these checks, we limit emergency sealing
// to the 100 lowest finalized blocks that are still unsealed.
// Inputs:
//   - `observer` for tracking and reporting the current internal state of the local sealing logic
//   - `lastFinalizedHeight` is the height of the latest block that is finalized
//   - `lastHeightWithFinalizedSeal` is the height of the latest block that is finalized and in addition
//
// No errors are expected during normal operations.
func (c *Core) checkEmergencySealing(observer consensus.SealingObservation, lastHeightWithFinalizedSeal, lastFinalizedHeight uint64) error {
	// if emergency sealing is not activated, then exit
	if !c.sealingConfigsGetter.EmergencySealingActiveConst() {
		return nil
	}

	// calculate total number of finalized blocks that are still unsealed
	if lastHeightWithFinalizedSeal > lastFinalizedHeight { // sanity check; protects calculation of `unsealedFinalizedCount` from underflow
		return fmt.Errorf(
			"latest finalized block must have height (%d) ≥ latest finalized _and_ sealed block (%d)", lastFinalizedHeight, lastHeightWithFinalizedSeal)
	}
	unsealedFinalizedCount := lastFinalizedHeight - lastHeightWithFinalizedSeal

	// We are checking emergency sealing only if there are more than approvals.DefaultEmergencySealingThresholdForFinalization
	// number of unsealed finalized blocks.
	if unsealedFinalizedCount <= approvals.DefaultEmergencySealingThresholdForFinalization {
		return nil
	}

	// we will check all the unsealed finalized height except the last approvals.DefaultEmergencySealingThresholdForFinalization
	// number of finalized heights
	heightCountForCheckingEmergencySealing := unsealedFinalizedCount - approvals.DefaultEmergencySealingThresholdForFinalization

	// If there are too many unsealed and finalized blocks, we don't have to check emergency sealing for all of them,
	// instead, only check for at most 100 blocks. This limits computation cost.
	// Note: the block builder also limits the max number of seals that can be included in a new block to `maxSealCount`.
	// While `maxSealCount` doesn't have to be the same value as the limit below, there is little benefit of our limit
	// exceeding `maxSealCount`.
	if heightCountForCheckingEmergencySealing > 100 {
		heightCountForCheckingEmergencySealing = 100
	}
	// if block is emergency sealable depends on it's incorporated block height
	// collectors tree stores collector by executed block height
	// we need to select multiple levels to find eligible collectors for emergency sealing
	for _, collector := range c.collectorTree.GetCollectorsByInterval(lastHeightWithFinalizedSeal, lastHeightWithFinalizedSeal+heightCountForCheckingEmergencySealing) {
		err := collector.CheckEmergencySealing(observer, lastFinalizedHeight)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Core) processPendingApprovals(collector approvals.AssignmentCollectorState) error {
	resultID := collector.ResultID()
	// filter cached approvals for concrete execution result
	for _, approval := range c.approvalsCache.TakeByResultID(resultID) {
		err := collector.ProcessApproval(approval)
		if err != nil {
			if engine.IsInvalidInputError(err) {
				c.log.Debug().
					Hex("result_id", resultID[:]).
					Err(err).
					Msgf("invalid approval with id %s", approval.ID())
			} else {
				return fmt.Errorf("could not process assignment: %w", err)
			}
		}
	}

	return nil
}

// ProcessFinalizedBlock processes finalization events in blocking way. The entire business
// logic in this function can be executed completely concurrently. We only waste some work
// if multiple goroutines enter the following block.
// Returns:
// * exception in case of unexpected error
// * nil - successfully processed finalized block
func (c *Core) ProcessFinalizedBlock(finalizedBlockID flow.Identifier) error {

	processFinalizedBlockSpan, _ := c.tracer.StartBlockSpan(context.Background(), finalizedBlockID, trace.CONSealingProcessFinalizedBlock)
	defer processFinalizedBlockSpan.End()

	// STEP 0: Collect auxiliary information
	// ------------------------------------------------------------------------
	// retrieve finalized block's header; update last finalized height and bail
	// if another goroutine is already ahead with a higher finalized block
	finalized, err := c.headers.ByBlockID(finalizedBlockID)
	if err != nil {
		return fmt.Errorf("could not retrieve header for finalized block %s", finalizedBlockID)
	}
	if !c.counterLastFinalizedHeight.Set(finalized.Height) {
		return nil
	}

	// retrieve latest _finalized_ seal in the fork with head finalizedBlock and update last
	// sealed height; we do _not_ bail, because we want to re-request approvals
	// especially, when sealing is stuck, i.e. last sealed height does not increase
	finalizedSeal, err := c.seals.HighestInFork(finalizedBlockID)
	if err != nil {
		return fmt.Errorf("could not retrieve finalizedSeal for finalized block %s", finalizedBlockID)
	}
	lastBlockWithFinalizedSeal, err := c.headers.ByBlockID(finalizedSeal.BlockID)
	if err != nil {
		return fmt.Errorf("could not retrieve last sealed block %v: %w", finalizedSeal.BlockID, err)
	}
	c.counterLastSealedHeight.Set(lastBlockWithFinalizedSeal.Height)

	// STEP 1: Pruning
	// ------------------------------------------------------------------------
	c.log.Info().Msgf("processing finalized block %v at height %d, lastSealedHeight %d", finalizedBlockID, finalized.Height, lastBlockWithFinalizedSeal.Height)
	err = c.prune(processFinalizedBlockSpan, finalized, lastBlockWithFinalizedSeal)
	if err != nil {
		return fmt.Errorf("updating to finalized block %v and sealed block %v failed: %w", finalizedBlockID, lastBlockWithFinalizedSeal.ID(), err)
	}

	// STEP 2: Check emergency sealing and re-request missing approvals
	// ------------------------------------------------------------------------
	sealingObservation := c.sealingTracker.NewSealingObservation(finalized, finalizedSeal, lastBlockWithFinalizedSeal)

	checkEmergencySealingSpan := c.tracer.StartSpanFromParent(processFinalizedBlockSpan, trace.CONSealingCheckForEmergencySealableBlocks)
	// check if there are stale results qualified for emergency sealing
	err = c.checkEmergencySealing(sealingObservation, lastBlockWithFinalizedSeal.Height, finalized.Height)
	checkEmergencySealingSpan.End()
	if err != nil {
		return fmt.Errorf("could not check emergency sealing at block %v", finalizedBlockID)
	}

	requestPendingApprovalsSpan := c.tracer.StartSpanFromParent(processFinalizedBlockSpan, trace.CONSealingRequestingPendingApproval)
	err = c.requestPendingApprovals(sealingObservation, lastBlockWithFinalizedSeal.Height, finalized.Height)
	requestPendingApprovalsSpan.End()
	if err != nil {
		return fmt.Errorf("internal error while requesting pending approvals: %w", err)
	}

	// While SealingObservation is not intrinsically concurrency safe, running the following operation
	// asynchronously is still safe for the following reason:
	// * The `sealingObservation` is thread-local: created and mutated only by this goroutine.
	// * According to the go spec: the statement that starts a new goroutine happens before the
	//   goroutine's execution begins. Hence, the goroutine executing the Complete() call
	//   observes the latest state of `sealingObservation`.
	// * The `sealingObservation` lives in the scope of this function. Hence, when this goroutine exits
	//   this function, `sealingObservation` lives solely in the scope of the newly-created goroutine.
	// We do this call asynchronously because we are in the hot path, and it is not required to progress,
	// and the call may involve database transactions that would unnecessarily delay sealing.
	c.reporter.reportAsync(sealingObservation)

	return nil
}

// prune updates the AssignmentCollectorTree's knowledge about sealed and finalized blocks.
// Furthermore, it  removes obsolete entries from AssignmentCollectorTree, RequestTracker
// and IncorporatedResultSeals mempool.
// We do _not_ expect any errors during normal operations.
func (c *Core) prune(parentSpan otelTrace.Span, finalized, lastSealed *flow.Header) error {
	pruningSpan := c.tracer.StartSpanFromParent(parentSpan, trace.CONSealingPruning)
	defer pruningSpan.End()

	err := c.collectorTree.FinalizeForkAtLevel(finalized, lastSealed) // stop collecting approvals for orphan collectors
	if err != nil {
		return fmt.Errorf("AssignmentCollectorTree failed to update its finalization state: %w", err)
	}

	err = c.requestTracker.PruneUpToHeight(lastSealed.Height)
	if err != nil && !mempool.IsBelowPrunedThresholdError(err) {
		return fmt.Errorf("could not request tracker at block up to height %d: %w", lastSealed.Height, err)
	}

	err = c.sealsMempool.PruneUpToHeight(lastSealed.Height) // prune candidate seals mempool
	if err != nil && !mempool.IsBelowPrunedThresholdError(err) {
		return fmt.Errorf("could not prune seals mempool at block up to height %d: %w", lastSealed.Height, err)
	}

	return nil
}

// requestPendingApprovals requests approvals for chunks that haven't collected
// enough approvals. When the number of unsealed finalized blocks exceeds the
// threshold, we go through the entire mempool of incorporated-results, which
// haven't yet been sealed, and check which chunks need more approvals. We only
// request approvals if the block incorporating the result is below the
// threshold.
//
//	                                  threshold
//	                             |                   |
//	... <-- A <-- A+1 <- ... <-- D <-- D+1 <- ... -- F
//	      sealed       maxHeightForRequesting      final
func (c *Core) requestPendingApprovals(observation consensus.SealingObservation, lastSealedHeight, lastFinalizedHeight uint64) error {
	if lastSealedHeight+c.sealingConfigsGetter.ApprovalRequestsThresholdConst() >= lastFinalizedHeight {
		return nil
	}

	// Reaching the following code implies:
	// 0 <= sealed.Height < final.Height - ApprovalRequestsThreshold
	// Hence, the following operation cannot underflow
	maxHeightForRequesting := lastFinalizedHeight - c.sealingConfigsGetter.ApprovalRequestsThresholdConst()

	pendingApprovalRequests := uint(0)
	collectors := c.collectorTree.GetCollectorsByInterval(lastSealedHeight, maxHeightForRequesting)
	for _, collector := range collectors {
		// Note:
		// * The `AssignmentCollectorTree` works with the height of the _executed_ block. However,
		//   the `maxHeightForRequesting` should use the height of the block _incorporating the result_
		//   as reference.
		// * There might be blocks whose height is below `maxHeightForRequesting`, while their result
		//   is incorporated into blocks with _larger_ height than `maxHeightForRequesting`. Therefore,
		//   filtering based on the executed block height is a useful pre-filter, but not quite
		//   precise enough.
		// * The `AssignmentCollector` will apply the precise filter to avoid unnecessary overhead.
		requestCount, err := collector.RequestMissingApprovals(observation, maxHeightForRequesting)
		if err != nil {
			return err
		}
		pendingApprovalRequests += requestCount
	}

	return nil
}

// getOutdatedBlockIDsFromRootSealingSegment finds all references to unknown blocks
// by execution results within the sealing segment. In general we disallow references
// to unknown blocks, but execution results incorporated within the sealing segment
// are an exception to this rule.
//
// For example, given the sealing segment A...E, B contains an ER referencing Z, but
// since Z is prior to sealing segment, the node cannot valid the ER. Therefore, we
// ignore these block references.
//
//	     [  sealing segment       ]
//	Z <- A <- B(RZ) <- C <- D <- E
func (c *Core) getOutdatedBlockIDsFromRootSealingSegment(rootHeader *flow.Header) (map[flow.Identifier]struct{}, error) {

	rootSealingSegment, err := c.state.AtBlockID(rootHeader.ID()).SealingSegment()
	if err != nil {
		return nil, fmt.Errorf("could not get root sealing segment: %w", err)
	}

	knownBlockIDs := make(map[flow.Identifier]struct{}) // track block IDs in the sealing segment
	outdatedBlockIDs := make(flow.IdentifierList, 0)
	for _, block := range rootSealingSegment.Blocks {
		knownBlockIDs[block.ID()] = struct{}{}
		for _, result := range block.Payload.Results {
			_, known := knownBlockIDs[result.BlockID]
			if !known {
				outdatedBlockIDs = append(outdatedBlockIDs, result.BlockID)
			}
		}
	}
	return outdatedBlockIDs.Lookup(), nil
}

// gatedSealingObservationReporter is a utility for gating asynchronous completion of sealing observations.
type gatedSealingObservationReporter struct {
	reporting *atomic.Bool // true when a sealing observation is actively being asynchronously completed
}

func newGatedSealingObservationReporter() *gatedSealingObservationReporter {
	return &gatedSealingObservationReporter{
		reporting: atomic.NewBool(false),
	}
}

// reportAsync only allows one in-flight observation completion at a time.
// Any extra observations are dropped.
func (reporter *gatedSealingObservationReporter) reportAsync(observation consensus.SealingObservation) {
	if reporter.reporting.CompareAndSwap(false, true) {
		go func() {
			observation.Complete()
			reporter.reporting.Store(false)
		}()
	}
}
