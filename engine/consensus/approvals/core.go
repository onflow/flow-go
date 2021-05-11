package approvals

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/consensus/sealing"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// ResultApprovalProcessor performs processing of execution results and result approvals.
// Accepts `flow.IncorporatedResult` to start processing approvals for particular result.
// Whenever enough approvals are collected produces a candidate seal and adds it to the mempool.
type ResultApprovalProcessor interface {
	// ProcessApproval processes approval in blocking way. Concurrency safe.
	// Returns:
	// * exception in case of unexpected error
	// * nil - successfully processed result approval
	ProcessApproval(approval *flow.ResultApproval) error
	// ProcessIncorporatedResult processes incorporated result in blocking way. Concurrency safe.
	// Returns:
	// * exception in case of unexpected error
	// * nil - successfully processed incorporated result
	ProcessIncorporatedResult(result *flow.IncorporatedResult) error
}

// approvalProcessingCore is an implementation of ResultApprovalProcessor interface
// This struct is responsible for:
// 	- collecting approvals for execution results
// 	- processing multiple incorporated results
// 	- pre-validating approvals (if they are outdated or non-verifiable)
// 	- pruning already processed collectorTree
type approvalProcessingCore struct {
	log                       zerolog.Logger           // used to log relevant actions with context
	collectorTree             *AssignmentCollectorTree // levelled forest for assignment collectors
	approvalsCache            *ApprovalsCache          // in-memory cache of approvals that weren't verified
	atomicLastSealedHeight    uint64                   // atomic variable for last sealed block height
	atomicLastFinalizedHeight uint64                   // atomic variable for last finalized block height
	emergencySealingActive    bool                     // flag which indicates if emergency sealing is active or not. NOTE: this is temporary while sealing & verification is under development
	headers                   storage.Headers          // used to access block headers in storage
	state                     protocol.State           // used to access protocol state
	seals                     storage.Seals            // used to get last sealed block
	requestTracker            *sealing.RequestTracker  // used to keep track of number of approval requests, and blackout periods, by chunk
}

func NewApprovalProcessingCore(headers storage.Headers, state protocol.State, sealsDB storage.Seals, assigner module.ChunkAssigner,
	verifier module.Verifier, sealsMempool mempool.IncorporatedResultSeals, approvalConduit network.Conduit, requiredApprovalsForSealConstruction uint, emergencySealingActive bool) (*approvalProcessingCore, error) {

	lastSealed, err := state.Sealed().Head()
	if err != nil {
		return nil, fmt.Errorf("could not retrieve last sealed block: %w", err)
	}

	core := &approvalProcessingCore{
		approvalsCache:         NewApprovalsCache(1000),
		headers:                headers,
		state:                  state,
		seals:                  sealsDB,
		emergencySealingActive: emergencySealingActive,
		requestTracker:         sealing.NewRequestTracker(10, 30),
	}

	factoryMethod := func(result *flow.ExecutionResult) (*AssignmentCollector, error) {
		return NewAssignmentCollector(result, core.state, core.headers, assigner, sealsMempool, verifier,
			approvalConduit, core.requestTracker, requiredApprovalsForSealConstruction)
	}

	core.collectorTree = NewAssignmentCollectorTree(lastSealed.Height, factoryMethod)

	return core, nil
}

func (c *approvalProcessingCore) lastSealedHeight() uint64 {
	return atomic.LoadUint64(&c.atomicLastSealedHeight)
}

func (c *approvalProcessingCore) lastFinalizedHeight() uint64 {
	return atomic.LoadUint64(&c.atomicLastFinalizedHeight)
}

// WARNING: this function is implemented in a way that we expect blocks strictly in parent-child order
// Caller has to ensure that it doesn't feed blocks that were already processed or in wrong order.
func (c *approvalProcessingCore) OnFinalizedBlock(finalizedBlockID flow.Identifier) {
	finalized, err := c.headers.ByBlockID(finalizedBlockID)
	if err != nil {
		c.log.Fatal().Err(err).Msgf("could not retrieve header for finalized block %s", finalizedBlockID)
	}

	// it's important to use atomic operation to make sure that we have correct ordering
	atomic.StoreUint64(&c.atomicLastFinalizedHeight, finalized.Height)

	seal, err := c.seals.ByBlockID(finalizedBlockID)
	if err != nil {
		c.log.Fatal().Err(err).Msgf("could not retrieve seal for finalized block %s", finalizedBlockID)
	}
	lastSealed, err := c.headers.ByBlockID(seal.BlockID)
	if err != nil {
		c.log.Fatal().Err(err).Msgf("could not retrieve last sealed block %s", seal.BlockID)
	}

	// it's important to use atomic operation to make sure that we have correct ordering
	atomic.StoreUint64(&c.atomicLastSealedHeight, lastSealed.Height)

	// check if there are stale results qualified for emergency sealing
	err = c.checkEmergencySealing(lastSealed.Height, finalized.Height)
	if err != nil {
		c.log.Fatal().Err(err).Msgf("could not check emergency sealing at block %v", finalizedBlockID)
	}

	// finalize forks to stop collecting approvals for orphan collectors
	c.collectorTree.FinalizeForkAtLevel(finalized.Height, finalizedBlockID)

	// as soon as we discover new sealed height, proceed with pruning collectors
	pruned, err := c.collectorTree.PruneUpToHeight(lastSealed.Height)
	if err != nil {
		c.log.Fatal().Err(err).Msgf("could not prune collectorTree tree at block %v", finalizedBlockID)
	}

	// remove all pending items that we might have requested
	c.requestTracker.Remove(pruned...)
}

// processIncorporatedResult implements business logic for processing single incorporated result
// Returns:
// * engine.InvalidInputError - incorporated result is invalid
// * engine.UnverifiableInputError - result is unverifiable since referenced block cannot be found
// * engine.OutdatedInputError - result is outdated for instance block was already sealed
// * exception in case of any other error, usually this is not expected
// * nil - successfully processed incorporated result
func (c *approvalProcessingCore) processIncorporatedResult(result *flow.IncorporatedResult) error {
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

	lastFinalizedBlockHeight := c.lastFinalizedHeight()

	// check if we are dealing with finalized block or an orphan
	if incorporatedAtHeight <= lastFinalizedBlockHeight {
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

	// in case block is not finalized we will create collector and start processing approvals
	// no checks for orphans can be made at this point
	// we expect that assignment collector will cleanup orphan IRs whenever new finalized block is processed

	lazyCollector, err := c.collectorTree.GetOrCreateCollector(result.Result)
	if err != nil {
		return fmt.Errorf("could not process incorporated result, cannot create collector: %w", err)
	}

	if lazyCollector.Orphan {
		return engine.NewOutdatedInputErrorf("collector for %s is marked as orphan", result.ID())
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
	if lazyCollector.Created {
		err = c.processPendingApprovals(lazyCollector.Collector)
		if err != nil {
			return fmt.Errorf("could not process cached approvals:  %w", err)
		}
	}

	return nil
}

func (c *approvalProcessingCore) ProcessIncorporatedResult(result *flow.IncorporatedResult) error {
	err := c.processIncorporatedResult(result)

	// we expect that only engine.UnverifiableInputError,
	// engine.OutdatedInputError, engine.InvalidInputError are expected, otherwise it's an exception
	if engine.IsUnverifiableInputError(err) || engine.IsOutdatedInputError(err) || engine.IsInvalidInputError(err) {
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
func (c *approvalProcessingCore) checkBlockOutdated(blockID flow.Identifier) error {
	block, err := c.headers.ByBlockID(blockID)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("failed to retrieve header for block %x: %w", blockID, err)
		}
		return engine.NewUnverifiableInputError("no header for block: %v", blockID)
	}

	// it's important to use atomic operation to make sure that we have correct ordering
	lastSealedHeight := c.lastSealedHeight()
	// drop approval, if it is for block whose height is lower or equal to already sealed height
	if lastSealedHeight >= block.Height {
		return engine.NewOutdatedInputErrorf("requested processing for already sealed block height")
	}

	return nil
}

func (c *approvalProcessingCore) ProcessApproval(approval *flow.ResultApproval) error {
	err := c.processApproval(approval)

	// we expect that only engine.UnverifiableInputError,
	// engine.OutdatedInputError, engine.InvalidInputError are expected, otherwise it's an exception
	if engine.IsUnverifiableInputError(err) || engine.IsOutdatedInputError(err) || engine.IsInvalidInputError(err) {
		return nil
	}

	return err
}

// processApproval implements business logic for processing single approval
// Returns:
// * engine.InvalidInputError - result approval is invalid
// * engine.UnverifiableInputError - result approval is unverifiable since referenced block cannot be found
// * engine.OutdatedInputError - result approval is outdated for instance block was already sealed
// * exception in case of any other error, usually this is not expected
// * nil - successfully processed result approval
func (c *approvalProcessingCore) processApproval(approval *flow.ResultApproval) error {
	err := c.checkBlockOutdated(approval.Body.BlockID)
	if err != nil {
		return fmt.Errorf("won't process approval for oudated block (%x): %w", approval.Body.BlockID, err)
	}

	if collector, orphan := c.collectorTree.GetCollector(approval.Body.ExecutionResultID); collector != nil {
		if orphan {
			return engine.NewOutdatedInputErrorf("collector for %s is marked as orphan", approval.Body.ExecutionResultID)
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

func (c *approvalProcessingCore) checkEmergencySealing(lastSealedHeight, lastFinalizedHeight uint64) error {
	if !c.emergencySealingActive {
		return nil
	}

	emergencySealingHeight := lastSealedHeight + sealing.DefaultEmergencySealingThreshold

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

func (c *approvalProcessingCore) processPendingApprovals(collector *AssignmentCollector) error {
	// filter cached approvals for concrete execution result
	for _, approvalID := range c.approvalsCache.ByResultID(collector.ResultID) {
		if approval := c.approvalsCache.Take(approvalID); approval != nil {
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
	}

	return nil
}
