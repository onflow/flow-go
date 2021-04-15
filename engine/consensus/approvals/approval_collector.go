package approvals

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool"
)

// ApprovalCollector is responsible for distributing work to chunk collectors,
// collecting aggregated signatures for chunks that reached seal construction threshold,
// creating and submitting seal candidates once signatures for every chunk are aggregated.
type ApprovalCollector struct {
	chunkCollectors      []*ChunkApprovalCollector
	aggregatedSignatures map[uint64]flow.AggregatedSignature // aggregated signature for each chunk
	lock                 sync.RWMutex                        // lock for modifying aggregatedSignatures
	incorporatedResult   *flow.IncorporatedResult
	numberOfChunks       int

	seals                                mempool.IncorporatedResultSeals // holds candidate seals for incorporated results that have acquired sufficient approvals; candidate seals are constructed  without consideration of the sealability of parent results
	requiredApprovalsForSealConstruction uint                            // min number of approvals required for constructing a candidate seal
}

func NewApprovalCollector(result *flow.IncorporatedResult, assignment *chunks.Assignment, seals mempool.IncorporatedResultSeals, requiredApprovalsForSealConstruction uint) *ApprovalCollector {
	chunkCollectors := make([]*ChunkApprovalCollector, 0, result.Result.Chunks.Len())
	for _, chunk := range result.Result.Chunks {
		chunkAssignment := make(map[flow.Identifier]struct{})
		for _, id := range assignment.Verifiers(chunk) {
			chunkAssignment[id] = struct{}{}
		}
		collector := NewChunkApprovalCollector(chunkAssignment)
		chunkCollectors = append(chunkCollectors, collector)
	}
	return &ApprovalCollector{
		incorporatedResult:                   result,
		numberOfChunks:                       result.Result.Chunks.Len(),
		chunkCollectors:                      chunkCollectors,
		requiredApprovalsForSealConstruction: requiredApprovalsForSealConstruction,
		aggregatedSignatures:                 make(map[uint64]flow.AggregatedSignature, result.Result.Chunks.Len()),
		seals:                                seals,
	}
}

func (c *ApprovalCollector) hasEnoughApprovals(chunkIndex uint64) bool {
	c.lock.RLock()
	defer c.lock.RUnlock()
	_, found := c.aggregatedSignatures[chunkIndex]
	return found
}

func (c *ApprovalCollector) sealResult() error {
	// get final state of execution result
	finalState, ok := c.incorporatedResult.Result.FinalStateCommitment()
	if !ok {
		// message correctness should have been checked before: failure here is an internal implementation bug
		return fmt.Errorf("failed to get final state commitment from Execution Result")
	}

	aggregatedSigs := make([]flow.AggregatedSignature, c.numberOfChunks)

	c.lock.RLock()
	for chunkIndex, sig := range c.aggregatedSignatures {
		aggregatedSigs[chunkIndex] = sig
	}
	c.lock.RUnlock()

	// TODO: Check SPoCK proofs

	// generate & store seal
	seal := &flow.Seal{
		BlockID:                c.incorporatedResult.Result.BlockID,
		ResultID:               c.incorporatedResult.Result.ID(),
		FinalState:             finalState,
		AggregatedApprovalSigs: aggregatedSigs,
	}

	// we don't care if the seal is already in the mempool
	_, err := c.seals.Add(&flow.IncorporatedResultSeal{
		IncorporatedResult: c.incorporatedResult,
		Seal:               seal,
	})
	if err != nil {
		return fmt.Errorf("failed to store IncorporatedResultSeal in mempool: %w", err)
	}

	return nil
}

func (c *ApprovalCollector) ProcessApproval(approval *flow.ResultApproval) error {
	chunkIndex := approval.Body.ChunkIndex
	if chunkIndex >= uint64(len(c.chunkCollectors)) {
		return engine.NewInvalidInputErrorf("approval collector chunk index out of range: %v", chunkIndex)
	}
	// there is no need to process approval if we have already enough info for sealing
	if c.hasEnoughApprovals(chunkIndex) {
		return nil
	}

	collector := c.chunkCollectors[chunkIndex]
	status := collector.ProcessApproval(approval)
	if status.numberOfApprovals >= c.requiredApprovalsForSealConstruction {
		c.collectAggregatedSignature(chunkIndex, collector)
		return c.trySealResult()
	}

	return nil
}

func (c *ApprovalCollector) trySealResult() error {
	c.lock.RLock()
	collectedSignatures := len(c.aggregatedSignatures)
	c.lock.RUnlock()

	if collectedSignatures < c.numberOfChunks {
		// some signatures are missing
		return nil
	}

	return c.sealResult()
}

func (c *ApprovalCollector) collectAggregatedSignature(chunkIndex uint64, collector *ChunkApprovalCollector) {
	if c.hasEnoughApprovals(chunkIndex) {
		return
	}

	aggregatedSignature := collector.GetAggregatedSignature()
	c.lock.Lock()
	defer c.lock.Unlock()
	c.aggregatedSignatures[chunkIndex] = aggregatedSignature
}
