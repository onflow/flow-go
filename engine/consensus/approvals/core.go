package approvals

import (
	"errors"
	"fmt"
	"github.com/onflow/flow-go/engine/consensus/sealing"
	"sync"
	"sync/atomic"

	lru "github.com/hashicorp/golang-lru"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/engine"
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
	// * engine.UnverifiableInputError -
	// * engine.OutdatedInputError - result approval is outdated, block was already sealed
	// * exception in case of any other error, usually this is not expected.
	// * nil - successfully processed result approval
	ProcessApproval(approval *flow.ResultApproval) error
	// ProcessIncorporatedResult processes incorporated result in blocking way, implementors need to ensure
	// that this function is reentrant.
	// Returns:
	// * engine.InvalidInputError - incorporated result is invalid
	// * engine.UnverifiableInputError - sentinel error in case we haven't discovered requested blockID
	// * engine.OutdatedInputError - sentinel error in case block is outdated
	// * exception in case of any other error, usually this is not expected.
	// * nil - successfully processed incorporated result
	ProcessIncorporatedResult(result *flow.IncorporatedResult) error
}

// approvalProcessingCore is an implementation of ResultApprovalProcessor interface
// This struct is responsible for:
// 	- collecting approvals for execution results
// 	- processing multiple incorporated results
// 	- pre-validating approvals(if they are outdated)
// 	- pruning already processed collectors
type approvalProcessingCore struct {
	collectors                           map[flow.Identifier]*AssignmentCollector // mapping of ResultID to AssignmentCollector
	lock                                 sync.RWMutex                             // lock for collectors
	approvalsCache                       *ApprovalsCache                          // in-memory cache of approvals that weren't verified
	blockHeightLookupCache               *lru.Cache                               // cache for block height lookups
	lastSealedBlockHeight                uint64                                   // atomic variable for last sealed block height
	requiredApprovalsForSealConstruction uint                                     // number of approvals that are required for each chunk to be sealed

	assigner        module.ChunkAssigner
	state           protocol.State
	verifier        module.Verifier
	seals           mempool.IncorporatedResultSeals
	payloads        storage.Payloads
	approvalConduit network.Conduit
	requestTracker  *sealing.RequestTracker
}

func NewApprovalProcessingCore(payloads storage.Payloads, state protocol.State, assigner module.ChunkAssigner,
	verifier module.Verifier, seals mempool.IncorporatedResultSeals, approvalConduit network.Conduit, requiredApprovalsForSealConstruction uint) *approvalProcessingCore {
	blockHeightLookupCache, _ := lru.New(100)
	return &approvalProcessingCore{
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
		blockHeightLookupCache:               blockHeightLookupCache,
		requestTracker:                       sealing.NewRequestTracker(10, 30),
	}
}

// WARNING: this function is implemented in a way that we expect blocks strictly in parent-child order
// Caller has to ensure that it doesn't feed blocks that were already processed or in wrong order.
func (p *approvalProcessingCore) OnFinalizedBlock(blockID flow.Identifier) {
	payload, err := p.payloads.ByBlockID(blockID)
	if err != nil {
		log.Fatal().Err(err).Msgf("could not retrieve payload for finalized block %s", blockID)
	}

	sealsCount := len(payload.Seals)
	sealedResultIds := make([]flow.Identifier, sealsCount)
	for i, seal := range payload.Seals {
		sealedResultIds[i] = seal.ResultID

		// update last sealed height
		if i == sealsCount-1 {
			head, err := p.state.AtBlockID(seal.BlockID).Head()
			if err != nil {
				log.Fatal().Err(err).Msgf("could not retrieve state for finalized block %s", seal.BlockID)
			}

			// it's important to use atomic operation to make sure that we have correct ordering
			atomic.StoreUint64(&p.lastSealedBlockHeight, head.Height)
		}
	}

	// cleanup collectors for already sealed results
	p.eraseCollectors(sealedResultIds)
}

func (p *approvalProcessingCore) ProcessIncorporatedResult(result *flow.IncorporatedResult) error {
	err := p.checkBlockOutdated(result.Result.BlockID)
	if err != nil {
		return fmt.Errorf("won't process outdated or unverifiable execution result %s: %w", result.Result.BlockID, err)
	}

	collector := p.getOrCreateCollector(result.Result.ID())
	err = collector.ProcessIncorporatedResult(result)
	if err != nil {
		return fmt.Errorf("could not process incorporated result: %w", err)
	}

	err = p.processPendingApprovals(collector)
	if err != nil {
		return fmt.Errorf("could not process cached approvals:  %w", err)
	}

	return nil
}

// checkBlockOutdated performs a sanity check if block is outdated
// Returns:
// * engine.UnverifiableInputError - sentinel error in case we haven't discovered requested blockID
// * engine.OutdatedInputError - sentinel error in case block is outdated
// * exception in case of unknown internal error
// * nil - block isn't sealed
func (p *approvalProcessingCore) checkBlockOutdated(blockID flow.Identifier) error {
	// approval validation is called for every approval
	// it's important to use a cache instead of looking up protocol.State for every approval
	// since we expect that there will be multiple approvals for same block
	// Peek internally used RWLock so it should be ok in terms of performance.
	height, cached := p.blockHeightLookupCache.Peek(blockID)
	if !cached {
		// check if we already have the block the approval pertains to
		head, err := p.state.AtBlockID(blockID).Head()
		if err != nil {
			if !errors.Is(err, storage.ErrNotFound) {
				return fmt.Errorf("failed to retrieve header for block %x: %w", blockID, err)
			}
			return engine.NewUnverifiableInputError("no header for block: %v", blockID)
		}

		height = head.Height
		p.blockHeightLookupCache.Add(blockID, height)
	}

	// it's important to use atomic operation to make sure that we have correct ordering
	lastSealedHeight := atomic.LoadUint64(&p.lastSealedBlockHeight)
	// drop approval, if it is for block whose height is lower or equal to already sealed height
	if lastSealedHeight >= height.(uint64) {
		return engine.NewOutdatedInputErrorf("result is for already sealed and finalized block height")
	}

	return nil
}

func (p *approvalProcessingCore) ProcessApproval(approval *flow.ResultApproval) error {
	err := p.checkBlockOutdated(approval.Body.BlockID)
	if err != nil {
		return err
	}

	if collector := p.getCollector(approval.Body.ExecutionResultID); collector != nil {
		// if there is a collector it means that we have received execution result and we are ready
		// to process approvals
		err = collector.ProcessAssignment(approval)
		if err != nil {
			return fmt.Errorf("could not process assignment: %w", err)
		}
	} else {
		// in case we haven't received execution result, cache it and process later.
		p.approvalsCache.Put(approval)
	}

	return nil
}

func (p *approvalProcessingCore) processPendingApprovals(collector *AssignmentCollector) error {
	predicate := func(approval *flow.ResultApproval) bool {
		return approval.Body.ExecutionResultID == collector.ResultID
	}

	// filter cached approvals for concrete execution result
	for _, approvalID := range p.approvalsCache.Ids() {
		if approval := p.approvalsCache.TakeIf(approvalID, predicate); approval != nil {
			err := collector.ProcessAssignment(approval)
			if err != nil {
				if engine.IsInvalidInputError(err) {
					log.Debug().
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

func (p *approvalProcessingCore) getCollector(resultID flow.Identifier) *AssignmentCollector {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.collectors[resultID]
}

func (p *approvalProcessingCore) createCollector(resultID flow.Identifier) *AssignmentCollector {
	p.lock.Lock()
	defer p.lock.Unlock()
	collector := NewAssignmentCollector(resultID, p.state, p.assigner, p.seals, p.verifier,
		p.approvalConduit, p.requestTracker, p.requiredApprovalsForSealConstruction)
	p.collectors[resultID] = collector
	return collector
}

func (p *approvalProcessingCore) eraseCollectors(resultIDs []flow.Identifier) {
	if len(resultIDs) == 0 {
		return
	}

	p.lock.Lock()
	defer p.lock.Unlock()
	for _, resultID := range resultIDs {
		delete(p.collectors, resultID)
	}
}

func (p *approvalProcessingCore) getOrCreateCollector(resultID flow.Identifier) *AssignmentCollector {
	if collector := p.getCollector(resultID); collector != nil {
		return collector
	}
	return p.createCollector(resultID)
}
