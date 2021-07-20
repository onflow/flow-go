// (c) 2021 Dapper Labs - ALL RIGHTS RESERVED

package sealing

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/consensus"
	"github.com/onflow/flow-go/engine/consensus/approvals"
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
// when set to 1, it requires at least 1 approval to build a seal
// when set to 0, it can build seal without any approval
const DefaultRequiredApprovalsForSealConstruction = 1

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
	unit                       *engine.Unit
	log                        zerolog.Logger                     // used to log relevant actions with context
	collectorTree              *approvals.AssignmentCollectorTree // levelled forest for assignment collectors
	approvalsCache             *approvals.LruCache                // in-memory cache of approvals that weren't verified
	counterLastSealedHeight    counters.StrictMonotonousCounter   // monotonous counter for last sealed block height
	counterLastFinalizedHeight counters.StrictMonotonousCounter   // monotonous counter for last finalized block height
	headers                    storage.Headers                    // used to access block headers in storage
	state                      protocol.State                     // used to access protocol state
	seals                      storage.Seals                      // used to get last sealed block
	sealsMempool               mempool.IncorporatedResultSeals    // used by tracker.SealingObservation to log info
	requestTracker             *approvals.RequestTracker          // used to keep track of number of approval requests, and blackout periods, by chunk
	metrics                    module.ConsensusMetrics            // used to track consensus metrics
	sealingTracker             consensus.SealingTracker           // logic-aware component for tracking sealing progress.
	tracer                     module.Tracer                      // used to trace execution
	config                     Config
}

func NewCore(
	log zerolog.Logger,
	tracer module.Tracer,
	conMetrics module.ConsensusMetrics,
	sealingTracker consensus.SealingTracker,
	unit *engine.Unit,
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
		sealingTracker:             sealingTracker,
		unit:                       unit,
		approvalsCache:             approvals.NewApprovalsLRUCache(1000),
		counterLastSealedHeight:    counters.NewMonotonousCounter(lastSealed.Height),
		counterLastFinalizedHeight: counters.NewMonotonousCounter(lastSealed.Height),
		headers:                    headers,
		state:                      state,
		seals:                      sealsDB,
		sealsMempool:               sealsMempool,
		config:                     config,
		requestTracker:             approvals.NewRequestTracker(headers, 10, 30),
	}

	factoryMethod := func(result *flow.ExecutionResult) (*approvals.AssignmentCollector, error) {
		return approvals.NewAssignmentCollector(core.log, result, core.state, core.headers, assigner, sealsMempool, verifier,
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

func (c *Core) checkEmergencySealing(observer consensus.SealingObservation, lastSealedHeight, lastFinalizedHeight uint64) error {
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
		err := collector.CheckEmergencySealing(observer, lastFinalizedHeight)
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

// ProcessFinalizedBlock processes finalization events in blocking way. The entire business
// logic in this function can be executed completely concurrently. We only waste some work
// if multiple goroutines enter the following block.
// Returns:
// * exception in case of unexpected error
// * nil - successfully processed finalized block
func (c *Core) ProcessFinalizedBlock(finalizedBlockID flow.Identifier) error {
	processFinalizedBlockSpan := c.tracer.StartSpan(finalizedBlockID, trace.CONSealingProcessFinalizedBlock)
	defer processFinalizedBlockSpan.Finish()

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

	// retrieve latest seal in the fork with head finalizedBlock and update last
	// sealed height; we do _not_ bail, because we want to re-request approvals
	// especially, when sealing is stuck, i.e. last sealed height does not increase
	seal, err := c.seals.ByBlockID(finalizedBlockID)
	if err != nil {
		return fmt.Errorf("could not retrieve seal for finalized block %s", finalizedBlockID)
	}
	lastSealed, err := c.headers.ByBlockID(seal.BlockID)
	if err != nil {
		return fmt.Errorf("could not retrieve last sealed block %v: %w", seal.BlockID, err)
	}
	c.counterLastSealedHeight.Set(lastSealed.Height)

	// STEP 1: Pruning
	// ------------------------------------------------------------------------
	c.log.Info().Msgf("processing finalized block %v at height %d, lastSealedHeight %d", finalizedBlockID, finalized.Height, lastSealed.Height)
	err = c.prune(processFinalizedBlockSpan, finalized, lastSealed)
	if err != nil {
		return fmt.Errorf("updating to finalized block %v and seald block %v failed: %w", finalizedBlockID, lastSealed.ID(), err)
	}

	// STEP 2: Check emergency sealing and re-request missing approvals
	// ------------------------------------------------------------------------
	sealingObservation := c.sealingTracker.NewSealingObservation(finalized, seal, lastSealed)

	checkEmergencySealingSpan := c.tracer.StartSpanFromParent(processFinalizedBlockSpan, trace.CONSealingCheckForEmergencySealableBlocks)
	// check if there are stale results qualified for emergency sealing
	err = c.checkEmergencySealing(sealingObservation, lastSealed.Height, finalized.Height)
	checkEmergencySealingSpan.Finish()
	if err != nil {
		return fmt.Errorf("could not check emergency sealing at block %v", finalizedBlockID)
	}

	requestPendingApprovalsSpan := c.tracer.StartSpanFromParent(processFinalizedBlockSpan, trace.CONSealingRequestingPendingApproval)
	err = c.requestPendingApprovals(sealingObservation, lastSealed.Height, finalized.Height)
	requestPendingApprovalsSpan.Finish()
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
	c.unit.Launch(sealingObservation.Complete)

	return nil
}

// prune updates the AssignmentCollectorTree's knowledge about sealed and finalized blocks.
// Furthermore, it  removes obsolete entries from AssignmentCollectorTree, RequestTracker
// and IncorporatedResultSeals mempool.
// We do _not_ expect any errors during normal operations.
func (c *Core) prune(parentSpan opentracing.Span, finalized, lastSealed *flow.Header) error {
	pruningSpan := c.tracer.StartSpanFromParent(parentSpan, trace.CONSealingPruning)
	defer pruningSpan.Finish()

	err := c.collectorTree.FinalizeForkAtLevel(finalized, lastSealed) // stop collecting approvals for orphan collectors
	if err != nil {
		return fmt.Errorf("AssignmentCollectorTree failed to update its finalization state: %w", err)
	}

	err = c.requestTracker.PruneUpToHeight(lastSealed.Height)
	if err != nil && !mempool.IsDecreasingPruningHeightError(err) {
		return fmt.Errorf("could not request tracker at block up to height %d: %w", lastSealed.Height, err)
	}

	err = c.sealsMempool.PruneUpToHeight(lastSealed.Height) // prune candidate seals mempool
	if err != nil && !mempool.IsDecreasingPruningHeightError(err) {
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
//                                   threshold
//                              |                   |
// ... <-- A <-- A+1 <- ... <-- D <-- D+1 <- ... -- F
//       sealed       maxHeightForRequesting      final
func (c *Core) requestPendingApprovals(observation consensus.SealingObservation, lastSealedHeight, lastFinalizedHeight uint64) error {
	if lastSealedHeight+c.config.ApprovalRequestsThreshold >= lastFinalizedHeight {
		return nil
	}

	// Reaching the following code implies:
	// 0 <= sealed.Height < final.Height - ApprovalRequestsThreshold
	// Hence, the following operation cannot underflow
	maxHeightForRequesting := lastFinalizedHeight - c.config.ApprovalRequestsThreshold

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
