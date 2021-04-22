package approvals

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	lru "github.com/hashicorp/golang-lru"
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
	// * engine.InvalidInputError - result approval is invalid
	// * engine.UnverifiableInputError - result approval is unverifiable since referenced block cannot be found
	// * engine.OutdatedInputError - result approval is outdated for instance block was already sealed
	// * exception in case of any other error, usually this is not expected
	// * nil - successfully processed result approval
	ProcessApproval(approval *flow.ResultApproval) error
	// ProcessIncorporatedResult processes incorporated result in blocking way, implementors need to ensure
	// that this function is reentrant.
	// Returns:
	// * engine.InvalidInputError - incorporated result is invalid
	// * engine.UnverifiableInputError - result is unverifiable since referenced block cannot be found
	// * engine.OutdatedInputError - result is outdated for instance block was already sealed
	// * exception in case of any other error, usually this is not expected
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
	blockHeightLookupCache               *lru.Cache                               // cache for block height lookups
	lastSealedBlockHeight                uint64                                   // atomic variable for last sealed block height
	requiredApprovalsForSealConstruction uint                                     // min number of approvals required for constructing a candidate seal
	emergencySealingActive               bool                                     // flag which indicates if emergency sealing is active or not. NOTE: this is temporary while sealing & verification is under development
	assigner                             module.ChunkAssigner                     // used by AssignmentCollector to build chunk assignment
	state                                protocol.State                           // used to access protocol state
	verifier                             module.Verifier                          // used to validate result approvals
	seals                                mempool.IncorporatedResultSeals          // holds candidate seals for incorporated results that have acquired sufficient approvals; candidate seals are constructed  without consideration of the sealability of parent results
	payloads                             storage.Payloads                         // used to access seals in finalized block
	approvalConduit                      network.Conduit                          // used to request missing approvals from verification nodes
	requestTracker                       *sealing.RequestTracker                  // used to keep track of number of approval requests, and blackout periods, by chunk
	getCachedBlockHeight                 GetCachedBlockHeight                     // cache height lookups
}

func NewApprovalProcessingCore(payloads storage.Payloads, state protocol.State, assigner module.ChunkAssigner,
	verifier module.Verifier, seals mempool.IncorporatedResultSeals, approvalConduit network.Conduit, requiredApprovalsForSealConstruction uint, emergencySealingActive bool) *approvalProcessingCore {
	blockHeightLookupCache, _ := lru.New(100)

	core := &approvalProcessingCore{
		collectors:                           make(map[flow.Identifier]*AssignmentCollector),
		approvalsCache:                       NewApprovalsCache(1000),
		lock:                                 sync.RWMutex{},
		assigner:                             assigner,
		state:                                state,
		verifier:                             verifier,
		seals:                                seals,
		payloads:                             payloads,
		approvalConduit:                      approvalConduit,
		requiredApprovalsForSealConstruction: requiredApprovalsForSealConstruction,
		emergencySealingActive:               emergencySealingActive,
		blockHeightLookupCache:               blockHeightLookupCache,
		requestTracker:                       sealing.NewRequestTracker(10, 30),
	}

	// Approval validation is called for every approval
	// it's important to use a cache instead of looking up protocol.State for every approval
	// since we expect that there will be multiple approvals for same block
	// Peek internally used RWLock so it should be ok in terms of performance.
	// Same functor is used by AssignmentCollector to reuse cache instead of using state for every
	// height lookup.
	core.getCachedBlockHeight = func(blockID flow.Identifier) (uint64, error) {
		height, cached := core.blockHeightLookupCache.Peek(blockID)
		if !cached {
			// check if we already have the block the approval pertains to
			head, err := core.state.AtBlockID(blockID).Head()
			if err != nil {
				if !errors.Is(err, storage.ErrNotFound) {
					return 0, fmt.Errorf("failed to retrieve header for block %x: %w", blockID, err)
				}
				return 0, engine.NewUnverifiableInputError("no header for block: %v", blockID)
			}

			height = head.Height
			core.blockHeightLookupCache.Add(blockID, height)
		}
		return height.(uint64), nil
	}

	return core
}

// WARNING: this function is implemented in a way that we expect blocks strictly in parent-child order
// Caller has to ensure that it doesn't feed blocks that were already processed or in wrong order.
func (c *approvalProcessingCore) OnFinalizedBlock(blockID flow.Identifier) {
	finalized, err := c.state.AtBlockID(blockID).Head()
	if err != nil {
		c.log.Fatal().Err(err).Msgf("could not retrieve header for finalized block %s", blockID)
	}

	payload, err := c.payloads.ByBlockID(blockID)
	if err != nil {
		c.log.Fatal().Err(err).Msgf("could not retrieve payload for finalized block %s", blockID)
	}

	sealsCount := len(payload.Seals)
	sealedResultIds := make([]flow.Identifier, sealsCount)
	lastSealedBlockHeight := uint64(0)
	for i, seal := range payload.Seals {
		sealedResultIds[i] = seal.ResultID

		// update last sealed height
		if i == sealsCount-1 {
			head, err := c.state.AtBlockID(seal.BlockID).Head()
			if err != nil {
				c.log.Fatal().Err(err).Msgf("could not retrieve state for finalized block %s", seal.BlockID)
			}

			lastSealedBlockHeight = head.Height

			// it's important to use atomic operation to make sure that we have correct ordering
			atomic.StoreUint64(&c.lastSealedBlockHeight, head.Height)
		}
	}

	// cleanup collectors for already sealed results
	c.eraseCollectors(sealedResultIds)
	collectors := c.allCollectors()

	// check if there are stale results qualified for emergency sealing
	err = c.checkEmergencySealing(collectors, finalized.Height)
	if err != nil {
		c.log.Err(err).Msgf("could not check emergency sealing at block %v", finalized.ID())
	}

	c.cleanupStaleCollectors(collectors, lastSealedBlockHeight)
}

func (c *approvalProcessingCore) ProcessIncorporatedResult(result *flow.IncorporatedResult) error {
	err := c.checkBlockOutdated(result.Result.BlockID)
	if err != nil {
		return fmt.Errorf("won't process outdated or unverifiable execution result %s: %w", result.Result.BlockID, err)
	}

	collector := c.getCollector(result.Result.ID())
	newIncorporatedResult := collector == nil
	if newIncorporatedResult {
		collector, err = c.createCollector(result.Result)
		if err != nil {
			return fmt.Errorf("could not process incorporated result, cannot create collector: %w", err)
		}
	}

	err = collector.ProcessIncorporatedResult(result)
	if err != nil {
		return fmt.Errorf("could not process incorporated result: %w", err)
	}

	// process pending approvals only if it's a new collector
	if newIncorporatedResult {
		err = c.processPendingApprovals(collector)
		if err != nil {
			return fmt.Errorf("could not process cached approvals:  %w", err)
		}
	}

	return nil
}

// checkBlockOutdated performs a sanity check if block is outdated
// Returns:
// * engine.UnverifiableInputError - sentinel error in case we haven't discovered requested blockID
// * engine.OutdatedInputError - sentinel error in case block is outdated
// * exception in case of unknown internal error
// * nil - block isn't sealed
func (c *approvalProcessingCore) checkBlockOutdated(blockID flow.Identifier) error {
	height, err := c.getCachedBlockHeight(blockID)
	if err != nil {
		return err
	}

	// it's important to use atomic operation to make sure that we have correct ordering
	lastSealedHeight := atomic.LoadUint64(&c.lastSealedBlockHeight)
	// drop approval, if it is for block whose height is lower or equal to already sealed height
	if lastSealedHeight >= height {
		return engine.NewOutdatedInputErrorf("result is for already sealed and finalized block height")
	}

	return nil
}

func (c *approvalProcessingCore) ProcessApproval(approval *flow.ResultApproval) error {
	err := c.checkBlockOutdated(approval.Body.BlockID)
	if err != nil {
		return err
	}

	if collector := c.getCollector(approval.Body.ExecutionResultID); collector != nil {
		// if there is a collector it means that we have received execution result and we are ready
		// to process approvals
		err = collector.ProcessAssignment(approval)
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
			err := collector.ProcessAssignment(approval)
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

func (c *approvalProcessingCore) createCollector(result *flow.ExecutionResult) (*AssignmentCollector, error) {
	resultID := result.ID()
	c.lock.Lock()
	defer c.lock.Unlock()
	collector, err := NewAssignmentCollector(result, c.state, c.assigner, c.seals, c.verifier, c.approvalConduit,
		c.requestTracker, c.getCachedBlockHeight, c.requiredApprovalsForSealConstruction)
	if err != nil {
		return nil, fmt.Errorf("could not create assignment collector for %v: %w", resultID, err)
	}
	c.collectors[resultID] = collector
	return collector, nil
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

func (c *approvalProcessingCore) cleanupStaleCollectors(collectors []*AssignmentCollector, lastSealedHeight uint64) {
	// We create collector only if we know about block that is being sealed. If we don't know anything about referred block
	// we will just discard it. This means even if someone tries to spam us with incorporated results with same blockID it
	// will get eventually cleaned when we discover a seal for this block.
	staleCollectors := make([]flow.Identifier, 0)
	for _, collector := range collectors {
		// we have collector for already sealed block
		if lastSealedHeight >= collector.BlockHeight {
			staleCollectors = append(staleCollectors, collector.ResultID)
		}
	}

	c.eraseCollectors(staleCollectors)
}
