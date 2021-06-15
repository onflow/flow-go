// (c) 2021 Dapper Labs - ALL RIGHTS RESERVED

package sealing

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/consensus/approvals"
	"github.com/onflow/flow-go/engine/consensus/approvals/tracker"
	"github.com/onflow/flow-go/engine/consensus/sealing/counters"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state/fork"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// DefaultRequiredApprovalsForSealConstruction is the default number of approvals required to construct a candidate seal
// for subsequent inclusion in block.
const DefaultRequiredApprovalsForSealConstruction = 0

// DefaultEmergencySealingActive is a flag which indicates when emergency sealing is active, this is a temporary measure
// to make fire fighting easier while seal & verification is under development.
const DefaultEmergencySealingActive = false

// Config is a structure of values that configure behavior of sealing engine
type Config struct {
	EmergencySealingActive               bool   // flag which indicates if emergency sealing is active or not. NOTE: this is temporary while sealing & verification is under development
	RequiredApprovalsForSealConstruction uint   // min number of approvals required for constructing a candidate seal
	ApprovalRequestsThreshold            uint64 // threshold for re-requesting approvals: min height difference between the latest finalized block and the block incorporating a result
}

func DefaultConfig() Config {
	return Config{
		EmergencySealingActive:               DefaultEmergencySealingActive,
		RequiredApprovalsForSealConstruction: DefaultRequiredApprovalsForSealConstruction,
		ApprovalRequestsThreshold:            10,
	}
}

// Core is an implementation of SealingCore interface
// This struct is responsible for:
// 	- collecting approvals for execution results
// 	- processing multiple incorporated results
// 	- pre-validating approvals (if they are outdated or non-verifiable)
// 	- pruning already processed collectorTree
type Core struct {
	log                        zerolog.Logger                     // used to log relevant actions with context
	collectorTree              *approvals.AssignmentCollectorTree // levelled forest for assignment collectors
	approvalsCache             *approvals.LruCache                // in-memory cache of approvals that weren't verified
	counterLastSealedHeight    counters.StrictMonotonousCounter   // monotonous counter for last sealed block height
	counterLastFinalizedHeight counters.StrictMonotonousCounter   // monotonous counter for last finalized block height
	headers                    storage.Headers                    // used to access block headers in storage
	state                      protocol.State                     // used to access protocol state
	seals                      storage.Seals                      // used to get last sealed block
	sealsMempool               mempool.IncorporatedResultSeals    // used by tracker.SealingTracker to log info
	requestTracker             *approvals.RequestTracker          // used to keep track of number of approval requests, and blackout periods, by chunk
	metrics                    module.ConsensusMetrics            // used to track consensus metrics
	tracer                     module.Tracer                      // used to trace execution
	config                     Config
}

func NewCore(
	log zerolog.Logger,
	tracer module.Tracer,
	conMetrics module.ConsensusMetrics,
	headers storage.Headers,
	state protocol.State,
	sealsDB storage.Seals,
	assigner module.ChunkAssigner,
	verifier module.Verifier,
	sealsMempool mempool.IncorporatedResultSeals,
	approvalConduit network.Conduit,
	config Config,
) (*Core, error) {
	lastSealed, err := state.Sealed().Head()
	if err != nil {
		return nil, fmt.Errorf("could not retrieve last sealed block: %w", err)
	}

	core := &Core{
		log:                        log.With().Str("engine", "sealing.Core").Logger(),
		tracer:                     tracer,
		metrics:                    conMetrics,
		approvalsCache:             approvals.NewApprovalsLRUCache(1000),
		counterLastSealedHeight:    counters.NewMonotonousCounter(lastSealed.Height),
		counterLastFinalizedHeight: counters.NewMonotonousCounter(lastSealed.Height),
		headers:                    headers,
		state:                      state,
		seals:                      sealsDB,
		sealsMempool:               sealsMempool,
		config:                     config,
		requestTracker:             approvals.NewRequestTracker(10, 30),
	}

	factoryMethod := func(result *flow.ExecutionResult) (*approvals.AssignmentCollector, error) {
		return approvals.NewAssignmentCollector(result, core.state, core.headers, assigner, sealsMempool, verifier,
			approvalConduit, core.requestTracker, config.RequiredApprovalsForSealConstruction)
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
	latestSeal, err := c.seals.ByBlockID(finalizedID)
	if err != nil {
		return fmt.Errorf("could not retrieve parent seal (%x): %w", finalizedID, err)
	}

	latestSealedBlockID := latestSeal.BlockID
	latestSealedBlock, err := c.headers.ByBlockID(latestSealedBlockID)
	if err != nil {
		return fmt.Errorf("could not retrieve latest sealed block (%x): %w", latestSealedBlockID, err)
	}

	// usually we start with empty collectors tree, prune it to minimum height
	_, err = c.collectorTree.PruneUpToHeight(latestSealedBlock.Height)
	if err != nil {
		return fmt.Errorf("could not prune execution tree to height %d: %w", latestSealedBlock.Height, err)
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
			// TODO: change this when migrating to sealing & verification phase 3.
			// Incorporated result is created this way only for phase 2.
			incorporatedResult := flow.NewIncorporatedResult(result.BlockID, result)
			err = c.ProcessIncorporatedResult(incorporatedResult)
			if err != nil {
				return fmt.Errorf("could not process incorporated result for block %s: %w", blockID, err)
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
	validPending, err := finalizedSnapshot.ValidDescendants()
	if err != nil {
		return fmt.Errorf("could not retrieve valid pending blocks from finalized snapshot: %w", err)
	}

	blocksProcessed = 0
	totalBlocks = uint64(len(validPending))

	c.log.Info().Msgf("reloading assignments from %d unfinalized blocks into collector tree", len(validPending))

	for _, blockID := range validPending {
		block, err := c.headers.ByBlockID(blockID)
		if err != nil {
			return fmt.Errorf("could not retrieve header for unfinalized block %x: %w", blockID, err)
		}
		err = resultProcessor(block)
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
func (c *Core) processIncorporatedResult(result *flow.IncorporatedResult) error {
	err := c.checkBlockOutdated(result.Result.BlockID)
	if err != nil {
		return fmt.Errorf("won't process outdated or unverifiable execution result %s: %w", result.Result.BlockID, err)
	}

	incorporatedBlock, err := c.headers.ByBlockID(result.IncorporatedBlockID)
	if err != nil {
		return fmt.Errorf("could not get block height for incorporated block %s: %w",
			result.IncorporatedBlockID, err)
	}
	incorporatedAtHeight := incorporatedBlock.Height

	// check if we are dealing with finalized block or an orphan
	if incorporatedAtHeight <= c.counterLastFinalizedHeight.Value() {
		finalized, err := c.headers.ByHeight(incorporatedAtHeight)
		if err != nil {
			return fmt.Errorf("could not retrieve finalized block at height %d: %w", incorporatedAtHeight, err)
		}
		if finalized.ID() != result.IncorporatedBlockID {
			// it means that we got incorporated result for a block which doesn't extend our chain
			// and should be discarded from future processing
			return engine.NewOutdatedInputErrorf("won't process incorporated result from orphan block %s", result.IncorporatedBlockID)
		}
	}

	// in case block is not finalized, we will create collector and start processing approvals
	// no checks for orphans can be made at this point
	// we expect that assignment collector will cleanup orphan IRs whenever new finalized block is processed
	lazyCollector, err := c.collectorTree.GetOrCreateCollector(result.Result)
	if err != nil {
		return fmt.Errorf("cannot create collector: %w", err)
	}

	err = lazyCollector.Collector.ProcessIncorporatedResult(result)
	if err != nil {
		return fmt.Errorf("could not process incorporated result: %w", err)
	}

	// process pending approvals only if it's a new collector
	// pending approvals are those we haven't received its result yet,
	// once we received a result and created a new collector, we find the pending
	// approvals for this result, and process them
	// newIncorporatedResult should be true only for one goroutine even if multiple access this code at the same
	// time, ensuring that processing of pending approvals happens once for particular assignment
	if lazyCollector.Created && lazyCollector.Processable {
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
	span := c.tracer.StartSpan(result.ID(), trace.CONSealingProcessIncorporatedResult)
	err := c.processIncorporatedResult(result)
	span.Finish()

	// We expect only engine.IsOutdatedInputError. If we encounter OutdatedInputError, InvalidInputError, we
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
	startTime := time.Now()
	approvalSpan := c.tracer.StartSpan(approval.ID(), trace.CONSealingProcessApproval)

	err := c.processApproval(approval)

	c.metrics.OnApprovalProcessingDuration(time.Since(startTime))
	approvalSpan.Finish()

	if err != nil {
		// only engine.UnverifiableInputError,
		// engine.OutdatedInputError, engine.InvalidInputError are expected, otherwise it's an exception
		if engine.IsUnverifiableInputError(err) || engine.IsOutdatedInputError(err) || engine.IsInvalidInputError(err) {
			logger := c.log.Info()
			if engine.IsInvalidInputError(err) {
				logger = c.log.Error()
			}

			logger.Err(err).
				Hex("approval_id", logging.Entity(approval)).
				Msgf("could not process result approval")

			return nil
		}

		marshalled, err := json.Marshal(approval)
		if err != nil {
			marshalled = []byte("json_marshalling_failed")
		}
		c.log.Error().Err(err).
			Hex("approval_id", logging.Entity(approval)).
			Str("approval", string(marshalled)).
			Msgf("unexpected error processing result approval")

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

	if collector, processable := c.collectorTree.GetCollector(approval.Body.ExecutionResultID); collector != nil {
		if !processable {
			return engine.NewOutdatedInputErrorf("collector for %s is marked as non processable", approval.Body.ExecutionResultID)
		}

		// if there is a collector it means that we have received execution result and we are ready
		// to process approvals
		err = collector.ProcessApproval(approval)
		if err != nil {
			return fmt.Errorf("could not process assignment: %w", err)
		}
	} else {
		// in case we haven't received execution result, cache it and process later.
		c.approvalsCache.Put(approval)
	}

	return nil
}

func (c *Core) checkEmergencySealing(lastSealedHeight, lastFinalizedHeight uint64) error {
	if !c.config.EmergencySealingActive {
		return nil
	}

	emergencySealingHeight := lastSealedHeight + approvals.DefaultEmergencySealingThreshold

	// we are interested in all collectors that match condition:
	// lastSealedBlock + sealing.DefaultEmergencySealingThreshold < lastFinalizedHeight
	// in other words we should check for emergency sealing only if threshold was reached
	if emergencySealingHeight >= lastFinalizedHeight {
		return nil
	}

	delta := lastFinalizedHeight - emergencySealingHeight
	// if block is emergency sealable depends on it's incorporated block height
	// collectors tree stores collector by executed block height
	// we need to select multiple levels to find eligible collectors for emergency sealing
	for _, collector := range c.collectorTree.GetCollectorsByInterval(lastSealedHeight, lastSealedHeight+delta) {
		err := collector.CheckEmergencySealing(lastFinalizedHeight)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Core) processPendingApprovals(collector *approvals.AssignmentCollector) error {
	// filter cached approvals for concrete execution result
	for _, approval := range c.approvalsCache.TakeByResultID(collector.ResultID) {
		err := collector.ProcessApproval(approval)
		if err != nil {
			if engine.IsInvalidInputError(err) {
				c.log.Debug().
					Hex("result_id", collector.ResultID[:]).
					Err(err).
					Msgf("invalid approval with id %s", approval.ID())
			} else {
				return fmt.Errorf("could not process assignment: %w", err)
			}
		}
	}

	return nil
}

// ProcessFinalizedBlock processes finalization events in blocking way. Concurrency safe.
// Returns:
// * exception in case of unexpected error
// * nil - successfully processed finalized block
func (c *Core) ProcessFinalizedBlock(finalizedBlockID flow.Identifier) error {
	processFinalizedBlockSpan := c.tracer.StartSpan(finalizedBlockID, trace.CONSealingProcessFinalizedBlock)
	defer processFinalizedBlockSpan.Finish()

	finalized, err := c.headers.ByBlockID(finalizedBlockID)
	if err != nil {
		return fmt.Errorf("could not retrieve header for finalized block %s", finalizedBlockID)
	}

	// update last finalized height, counter will return false if there is already a bigger value
	if !c.counterLastFinalizedHeight.Set(finalized.Height) {
		return nil
	}

	seal, err := c.seals.ByBlockID(finalizedBlockID)
	if err != nil {
		return fmt.Errorf("could not retrieve seal for finalized block %s", finalizedBlockID)
	}
	lastSealed, err := c.headers.ByBlockID(seal.BlockID)
	if err != nil {
		c.log.Fatal().Err(err).Msgf("could not retrieve last sealed block %s", seal.BlockID)
	}

	c.log.Info().Msgf("processing finalized block %v at height %d, lastSealedHeight %d", finalizedBlockID, finalized.Height, lastSealed.Height)

	c.counterLastSealedHeight.Set(lastSealed.Height)

	checkEmergencySealingSpan := c.tracer.StartSpanFromParent(processFinalizedBlockSpan, trace.CONSealingCheckForEmergencySealableBlocks)
	// check if there are stale results qualified for emergency sealing
	err = c.checkEmergencySealing(lastSealed.Height, finalized.Height)
	checkEmergencySealingSpan.Finish()
	if err != nil {
		return fmt.Errorf("could not check emergency sealing at block %v", finalizedBlockID)
	}

	updateCollectorTreeSpan := c.tracer.StartSpanFromParent(processFinalizedBlockSpan, trace.CONSealingUpdateAssignmentCollectorTree)
	// finalize forks to stop collecting approvals for orphan collectors
	err = c.collectorTree.FinalizeForkAtLevel(finalized, lastSealed)
	if err != nil {
		updateCollectorTreeSpan.Finish()
		return fmt.Errorf("collectors tree could not finalize fork: %w", err)
	}

	pruned, err := c.collectorTree.PruneUpToHeight(lastSealed.Height)
	if err != nil {
		return fmt.Errorf("could not prune collectorTree tree at block %v", finalizedBlockID)
	}
	c.requestTracker.Remove(pruned...) // remove all pending items that we might have requested
	updateCollectorTreeSpan.Finish()

	err = c.sealsMempool.PruneUpToHeight(lastSealed.Height)
	if err != nil && !mempool.IsDecreasingPruningHeightError(err) {
		return fmt.Errorf("could not prune seals mempool at block %v, by height: %v: %w", finalizedBlockID, lastSealed.Height, err)
	}

	requestPendingApprovalsSpan := c.tracer.StartSpanFromParent(processFinalizedBlockSpan, trace.CONSealingRequestingPendingApproval)
	err = c.requestPendingApprovals(lastSealed.Height, finalized.Height)
	requestPendingApprovalsSpan.Finish()
	if err != nil {
		return fmt.Errorf("internal error while requesting pending approvals: %w", err)
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
//                                   threshold
//                              |                   |
// ... <-- A <-- A+1 <- ... <-- D <-- D+1 <- ... -- F
//       sealed       maxHeightForRequesting      final
func (c *Core) requestPendingApprovals(lastSealedHeight, lastFinalizedHeight uint64) error {
	// skip requesting approvals if they are not required for sealing
	if c.config.RequiredApprovalsForSealConstruction == 0 {
		return nil
	}

	if lastSealedHeight+c.config.ApprovalRequestsThreshold >= lastFinalizedHeight {
		return nil
	}

	startTime := time.Now()
	sealingTracker := tracker.NewSealingTracker(c.state)

	// Reaching the following code implies:
	// 0 <= sealed.Height < final.Height - ApprovalRequestsThreshold
	// Hence, the following operation cannot underflow
	maxHeightForRequesting := lastFinalizedHeight - c.config.ApprovalRequestsThreshold

	pendingApprovalRequests := 0
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
		requestCount, err := collector.RequestMissingApprovals(sealingTracker, maxHeightForRequesting)
		if err != nil {
			return err
		}
		pendingApprovalRequests += requestCount
	}

	c.log.Info().
		Str("next_unsealed_results", sealingTracker.String()).
		Bool("mempool_has_seal_for_next_height", sealingTracker.MempoolHasNextSeal(c.sealsMempool)).
		Uint("seals_size", c.sealsMempool.Size()).
		Uint64("last_sealed_height", lastSealedHeight).
		Uint64("last_finalized_height", lastFinalizedHeight).
		Int("pending_collectors", len(collectors)).
		Int("pending_approval_requests", pendingApprovalRequests).
		Int64("duration_ms", time.Since(startTime).Milliseconds()).
		Msg("requested pending approvals successfully")

	return nil
}
