package approvals

import (
	"errors"
	"fmt"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"sync"
	"sync/atomic"

	lru "github.com/hashicorp/golang-lru"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// ResultApprovalProcessor performs processing and storing of approvals
type ResultApprovalProcessor interface {
	// submits approval for processing in sync way
	ProcessApproval(approval *flow.ResultApproval) error
	// submits incorporated result for processing in sync way
	ProcessIncorporatedResult(result *flow.IncorporatedResult) error
}

// implementation of ResultApprovalProcessor
type approvalProcessingCore struct {
	collectors                           map[flow.Identifier]*AssignmentCollector // contains a mapping of ResultID to AssignmentCollector
	lock                                 sync.RWMutex                             // lock for collectors
	assigner                             module.ChunkAssigner
	state                                protocol.State
	verifier                             module.Verifier
	seals                                mempool.IncorporatedResultSeals
	payloads                             storage.Payloads
	lastSealedBlockHeight                uint64 // atomic variable for last sealed block height
	blockHeightLookupCache               *lru.Cache
	requiredApprovalsForSealConstruction uint
}

func NewApprovalProcessingCore(payloads storage.Payloads, state protocol.State, assigner module.ChunkAssigner,
	verifier module.Verifier, seals mempool.IncorporatedResultSeals, requiredApprovalsForSealConstruction uint) *approvalProcessingCore {
	blockHeightLookupCache, _ := lru.New(100)
	return &approvalProcessingCore{
		collectors:                           make(map[flow.Identifier]*AssignmentCollector),
		lock:                                 sync.RWMutex{},
		assigner:                             assigner,
		state:                                state,
		verifier:                             verifier,
		seals:                                seals,
		payloads:                             payloads,
		requiredApprovalsForSealConstruction: requiredApprovalsForSealConstruction,
		blockHeightLookupCache:               blockHeightLookupCache,
	}
}

func (p *approvalProcessingCore) OnFinalizedBlock(block *model.Block) {
	payload, err := p.payloads.ByBlockID(block.BlockID)
	if err != nil {
		log.Fatal().Err(err).Msgf("could not retrieve payload for finalized block %s", block.BlockID)
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
			atomic.StoreUint64(&p.lastSealedBlockHeight, head.Height)
		}
	}

	// cleanup collectors for already sealed results
	p.eraseCollectors(sealedResultIds)
}

func (p *approvalProcessingCore) ProcessIncorporatedResult(result *flow.IncorporatedResult) error {
	collector := p.getOrCreateCollector(result.Result.ID())
	err := collector.ProcessIncorporatedResult(result)
	if err != nil {
		return fmt.Errorf("could not process incorporated result: %w", err)
	}
	return nil
}

func (p *approvalProcessingCore) validateApproval(approval *flow.ResultApproval) error {
	blockID := approval.Body.BlockID
	height, cached := p.blockHeightLookupCache.Get(blockID)
	if !cached {
		// check if we already have the block the approval pertains to
		head, err := p.state.AtBlockID(blockID).Head()
		if err != nil {
			if !errors.Is(err, storage.ErrNotFound) {
				return fmt.Errorf("failed to retrieve header for block %x: %w", approval.Body.BlockID, err)
			}
			return engine.NewUnverifiableInputError("no header for block: %v", approval.Body.BlockID)
		}

		height = head.Height
		p.blockHeightLookupCache.Add(blockID, height)
	}

	lastSealedHeight := atomic.LoadUint64(&p.lastSealedBlockHeight)
	// drop approval, if it is for block whose height is lower or equal to already sealed height
	if lastSealedHeight >= height.(uint64) {
		return engine.NewOutdatedInputErrorf("result is for already sealed and finalized block height")
	}

	return nil
}

func (p *approvalProcessingCore) ProcessApproval(approval *flow.ResultApproval) error {
	err := p.validateApproval(approval)
	if err != nil {
		return err
	}

	collector := p.getOrCreateCollector(approval.Body.ExecutionResultID)
	err = collector.ProcessAssignment(approval)
	if err != nil {
		return fmt.Errorf("could not process assignment: %w", err)
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
		p.requiredApprovalsForSealConstruction)
	p.collectors[resultID] = collector
	return collector
}

func (p *approvalProcessingCore) eraseCollectors(resultIDs []flow.Identifier) {
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
