package approvals

import (
	"errors"
	"fmt"
	"sync"
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
	// ProcessApproval processes approval in blocking way, implementors need to ensure
	// that this function is reentrant and can be safely used in concurrent environment.
	// Returns:
	// * exception in case of unexpected error
	// * nil - successfully processed result approval
	ProcessApproval(approval *flow.ResultApproval) error
	// ProcessIncorporatedResult processes incorporated result in blocking way, implementors need to ensure
	// that this function is reentrant.
	// Returns:
	// * exception in case of unexpected error
	// * nil - successfully processed incorporated result
	ProcessIncorporatedResult(result *flow.IncorporatedResult) error
}

// approvalProcessingCore is an implementation of ResultApprovalProcessor interface
// This struct is responsible for:
// 	- collecting approvals for execution results
// 	- processing multiple incorporated results
// 	- pre-validating approvals(if they are outdated or non-verifiable)
// 	- pruning already processed collectors
type approvalProcessingCore struct {
	log                                  zerolog.Logger                           // used to log relevant actions with context
	collectors                           map[flow.Identifier]*AssignmentCollector // mapping of ResultID to AssignmentCollector
	lock                                 sync.RWMutex                             // lock for collectors
	approvalsCache                       *ApprovalsCache                          // in-memory cache of approvals that weren't verified
	atomicLastSealedHeight               uint64                                   // atomic variable for last sealed block height
	atomicLastFinalizedHeight            uint64                                   // atomic variable for last finalized block height
	requiredApprovalsForSealConstruction uint                                     // min number of approvals required for constructing a candidate seal
	emergencySealingActive               bool                                     // flag which indicates if emergency sealing is active or not. NOTE: this is temporary while sealing & verification is under development
	assigner                             module.ChunkAssigner                     // used by AssignmentCollector to build chunk assignment
	headers                              storage.Headers                          // used to access headers in storage
	state                                protocol.State                           // used to access protocol state
	verifier                             module.Verifier                          // used to validate result approvals
	seals                                mempool.IncorporatedResultSeals          // holds candidate seals for incorporated results that have acquired sufficient approvals; candidate seals are constructed  without consideration of the sealability of parent results
	payloads                             storage.Payloads                         // used to access seals in finalized block
	approvalConduit                      network.Conduit                          // used to request missing approvals from verification nodes
	requestTracker                       *sealing.RequestTracker                  // used to keep track of number of approval requests, and blackout periods, by chunk
}

func NewApprovalProcessingCore(headers storage.Headers, payloads storage.Payloads, state protocol.State, assigner module.ChunkAssigner,
	verifier module.Verifier, seals mempool.IncorporatedResultSeals, approvalConduit network.Conduit, requiredApprovalsForSealConstruction uint, emergencySealingActive bool) *approvalProcessingCore {

	core := &approvalProcessingCore{
		collectors:                           make(map[flow.Identifier]*AssignmentCollector),
		approvalsCache:                       NewApprovalsCache(1000),
		lock:                                 sync.RWMutex{},
		assigner:                             assigner,
		headers:                              headers,
		state:                                state,
		verifier:                             verifier,
		seals:                                seals,
		payloads:                             payloads,
		approvalConduit:                      approvalConduit,
		requiredApprovalsForSealConstruction: requiredApprovalsForSealConstruction,
		emergencySealingActive:               emergencySealingActive,
		requestTracker:                       sealing.NewRequestTracker(10, 30),
	}

	return core
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

	payload, err := c.payloads.ByBlockID(finalizedBlockID)
	if err != nil {
		c.log.Fatal().Err(err).Msgf("could not retrieve payload for finalized block %s", finalizedBlockID)
	}

	// it's important to use atomic operation to make sure that we have correct ordering
	atomic.StoreUint64(&c.atomicLastFinalizedHeight, finalized.Height)

	lastSealed, err := c.state.Sealed().Head()
	if err != nil {
		c.log.Fatal().Err(err).Msgf("could not retrieve last sealed block")
	}

	// it's important to use atomic operation to make sure that we have correct ordering
	atomic.StoreUint64(&c.atomicLastSealedHeight, lastSealed.Height)

	sealsCount := len(payload.Seals)
	sealedResultIds := make([]flow.Identifier, sealsCount)
	for i, seal := range payload.Seals {
		sealedResultIds[i] = seal.ResultID
	}

	// cleanup collectors for already sealed results
	c.eraseCollectors(sealedResultIds)
	collectors := c.allCollectors()

	// check if there are stale results qualified for emergency sealing
	err = c.checkEmergencySealing(collectors, finalized.Height)
	if err != nil {
		c.log.Fatal().Err(err).Msgf("could not check emergency sealing at block %v", finalizedBlockID)
	}

	collectors = c.cleanupStaleCollectors(collectors, lastSealed.Height)
	// to those collectors that are not stale, report finalization event to cleanup orphan blocks
	for _, collector := range collectors {
		collector.OnBlockFinalizedAtHeight(finalizedBlockID, finalized.Height)
	}
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

	collector, newIncorporatedResult, err := c.getOrCreateCollector(result.Result)
	if err != nil {
		return fmt.Errorf("could not process incorporated result, cannot create collector: %w", err)
	}

	err = collector.ProcessIncorporatedResult(result)
	if err != nil {
		return fmt.Errorf("could not process incorporated result: %w", err)
	}

	// process pending approvals only if it's a new collector
	// pending approvals are those we haven't received its result yet,
	// once we received a result and created a new collector, we find the pending
	// approvals for this result, and process them
	// newIncorporatedResult should be true only for one goroutine even if multiple access this code at the same
	// time, ensuring that processing of pending approvals happens once for particular assignment
	if newIncorporatedResult {
		err = c.processPendingApprovals(collector)
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

	if collector := c.getCollector(approval.Body.ExecutionResultID); collector != nil {
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

func (c *approvalProcessingCore) checkEmergencySealing(collectors []*AssignmentCollector, lastFinalizedHeight uint64) error {
	if !c.emergencySealingActive {
		return nil
	}

	for _, collector := range collectors {
		// let's check at least that candidate block is deep enough for emergency sealing.
		if collector.BlockHeight+sealing.DefaultEmergencySealingThreshold < lastFinalizedHeight {
			err := collector.CheckEmergencySealing(lastFinalizedHeight)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *approvalProcessingCore) processPendingApprovals(collector *AssignmentCollector) error {
	predicate := func(approval *flow.ResultApproval) bool {
		return approval.Body.ExecutionResultID == collector.ResultID
	}

	// filter cached approvals for concrete execution result
	for _, approvalID := range c.approvalsCache.Ids() {
		if approval := c.approvalsCache.TakeIf(approvalID, predicate); approval != nil {
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

func (c *approvalProcessingCore) allCollectors() []*AssignmentCollector {
	collectors := make([]*AssignmentCollector, 0, len(c.collectors))
	c.lock.RLock()
	defer c.lock.RUnlock()
	for _, collector := range c.collectors {
		collectors = append(collectors, collector)
	}
	return collectors
}

func (c *approvalProcessingCore) getCollector(resultID flow.Identifier) *AssignmentCollector {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.collectors[resultID]
}

// getOrCreateCollector performs lazy initialization of AssignmentCollector using double checked locking
// Returns, (AssignmentCollector, true or false whenever it was created, error)
func (c *approvalProcessingCore) getOrCreateCollector(result *flow.ExecutionResult) (*AssignmentCollector, bool, error) {
	resultID := result.ID()
	// first let's check if we have a collector already
	collector := c.getCollector(resultID)
	if collector != nil {
		return collector, false, nil
	}

	// fast check shows that there is no collector, need to create one
	c.lock.Lock()
	defer c.lock.Unlock()

	// we need to check again, since it's possible that after checking for existing collector but before taking a lock
	// new collector was created by concurrent goroutine
	collector, ok := c.collectors[resultID]
	if ok {
		return collector, false, nil
	}

	collector, err := NewAssignmentCollector(result, c.state, c.headers, c.assigner, c.seals, c.verifier, c.approvalConduit,
		c.requestTracker, c.requiredApprovalsForSealConstruction)
	if err != nil {
		return nil, false, fmt.Errorf("could not create assignment collector for %v: %w", resultID, err)
	}
	c.collectors[resultID] = collector
	return collector, true, nil
}

func (c *approvalProcessingCore) eraseCollectors(resultIDs []flow.Identifier) {
	if len(resultIDs) == 0 {
		return
	}

	c.lock.Lock()
	defer c.lock.Unlock()
	for _, resultID := range resultIDs {
		delete(c.collectors, resultID)
	}
}

func (c *approvalProcessingCore) cleanupStaleCollectors(collectors []*AssignmentCollector, lastSealedHeight uint64) []*AssignmentCollector {
	// We create collector only if we know about block that is being sealed. If we don't know anything about referred block
	// we will just discard it. This means even if someone tries to spam us with incorporated results with same blockID it
	// will get eventually cleaned when we discover a seal for this block.
	staleCollectors := make([]flow.Identifier, 0)
	filteredCollectors := make([]*AssignmentCollector, 0)
	for _, collector := range collectors {
		// we have collector for already sealed block
		if lastSealedHeight >= collector.BlockHeight {
			staleCollectors = append(staleCollectors, collector.ResultID)
		} else {
			filteredCollectors = append(filteredCollectors, collector)
		}
	}

	c.eraseCollectors(staleCollectors)

	return filteredCollectors
}
