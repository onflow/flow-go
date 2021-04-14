package approvals

import (
	"errors"
	"fmt"
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
type resultsApprovalProcessor struct {
	collectors                           map[flow.Identifier]*AssignmentCollector // contains a mapping of ResultID to AssignmentCollector
	lock                                 sync.RWMutex                             // lock for collectors
	assigner                             module.ChunkAssigner
	state                                protocol.State
	verifier                             module.Verifier
	seals                                mempool.IncorporatedResultSeals
	lastSealedBlockHeight                uint64 // atomic variable for last sealed block height
	blockHeightLookupCache               *lru.Cache
	requiredApprovalsForSealConstruction uint
}

func NewResultsApprovalProcess(state protocol.State, assigner module.ChunkAssigner,
	verifier module.Verifier, seals mempool.IncorporatedResultSeals, requiredApprovalsForSealConstruction uint) *resultsApprovalProcessor {
	blockHeightLookupCache, _ := lru.New(100)
	return &resultsApprovalProcessor{
		collectors:                           make(map[flow.Identifier]*AssignmentCollector),
		lock:                                 sync.RWMutex{},
		assigner:                             assigner,
		state:                                state,
		verifier:                             verifier,
		seals:                                seals,
		requiredApprovalsForSealConstruction: requiredApprovalsForSealConstruction,
		blockHeightLookupCache:               blockHeightLookupCache,
	}
}

func (p *resultsApprovalProcessor) OnBlockFinalized(block flow.Block) {
	sealsCount := len(block.Payload.Seals)
	sealedResultIds := make([]flow.Identifier, sealsCount)
	for i, seal := range block.Payload.Seals {
		sealedResultIds[i] = seal.ResultID

		// update last sealed height
		if i == sealsCount-1 {
			head, err := p.state.AtBlockID(seal.BlockID).Head()
			if err != nil {
				log.Fatal().Msgf("could not retrieve state of finalized block %s: %w", seal.BlockID, err)
			}
			atomic.StoreUint64(&p.lastSealedBlockHeight, head.Height)
		}
	}

	// cleanup collectors for already sealed results
	p.eraseCollectors(sealedResultIds)
}

func (p *resultsApprovalProcessor) ProcessIncorporatedResult(result *flow.IncorporatedResult) error {
	collector := p.getOrCreateCollector(result.Result.ID())
	err := collector.ProcessIncorporatedResult(result)
	if err != nil {
		return fmt.Errorf("could not process incorporated result: %w", err)
	}
	return nil
}

func (p *resultsApprovalProcessor) validateApproval(approval *flow.ResultApproval) error {
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

		p.blockHeightLookupCache.Add(blockID, head.Height)
	}

	lastSealedHeight := atomic.LoadUint64(&p.lastSealedBlockHeight)
	// drop approval, if it is for block whose height is lower or equal to already sealed height
	if lastSealedHeight >= height.(uint64) {
		return engine.NewOutdatedInputErrorf("result is for already sealed and finalized block height")
	}

	return nil
}

func (p *resultsApprovalProcessor) ProcessApproval(approval *flow.ResultApproval) error {
	collector := p.getOrCreateCollector(approval.Body.ExecutionResultID)
	err := collector.ProcessAssignment(approval)
	if err != nil {
		return fmt.Errorf("could not process assignment: %w", err)
	}
	return nil
}

func (p *resultsApprovalProcessor) getCollector(resultID flow.Identifier) *AssignmentCollector {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.collectors[resultID]
}

func (p *resultsApprovalProcessor) createCollector(resultID flow.Identifier) *AssignmentCollector {
	p.lock.Lock()
	defer p.lock.Unlock()
	collector := NewAssignmentCollector(resultID, p.state, p.assigner, p.seals, p.verifier,
		p.requiredApprovalsForSealConstruction)
	p.collectors[resultID] = collector
	return collector
}

func (p *resultsApprovalProcessor) eraseCollectors(resultIDs []flow.Identifier) {
	p.lock.Lock()
	defer p.lock.Unlock()
	for _, resultID := range resultIDs {
		delete(p.collectors, resultID)
	}
}

func (p *resultsApprovalProcessor) getOrCreateCollector(resultID flow.Identifier) *AssignmentCollector {
	if collector := p.getCollector(resultID); collector != nil {
		return collector
	}
	return p.createCollector(resultID)
}
