package approvals

import (
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"sync"
)

type ApprovalCollector struct {
	incorporatedBlockID                  flow.Identifier
	chunkCollectors                      []*ChunkApprovalCollector
	assignmentAuthorizedVerifiers        map[flow.Identifier]struct{}
	requiredApprovalsForSealConstruction uint                       // min number of approvals required for constructing a candidate seal
	aggregatedSignatures                 []flow.AggregatedSignature // aggregated signature for each chunk
	lock                                 sync.RWMutex               // lock for modifying aggregatedSignatures
}

func NewApprovalCollector(result *flow.IncorporatedResult, assignment *chunks.Assignment,
	authorizedVerifiers map[flow.Identifier]struct{}, requiredApprovalsForSealConstruction uint) *ApprovalCollector {
	chunkCollectors := make([]*ChunkApprovalCollector, 0, result.Result.Chunks.Len())
	for _, chunk := range result.Result.Chunks {
		chunkAssignment := make(map[flow.Identifier]struct{})
		for _, id := range assignment.Verifiers(chunk) {
			chunkAssignment[id] = struct{}{}
		}
		collector := NewChunkApprovalCollector(chunkAssignment, authorizedVerifiers)
		chunkCollectors = append(chunkCollectors, collector)
	}
	return &ApprovalCollector{
		incorporatedBlockID:                  result.IncorporatedBlockID,
		chunkCollectors:                      chunkCollectors,
		assignmentAuthorizedVerifiers:        authorizedVerifiers,
		requiredApprovalsForSealConstruction: requiredApprovalsForSealConstruction,
		aggregatedSignatures:                 make([]flow.AggregatedSignature, result.Result.Chunks.Len()),
	}
}

func (c *ApprovalCollector) hasEnoughApprovals(chunkIndex uint64) bool {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return uint(c.aggregatedSignatures[chunkIndex].NumberSigners()) >= c.requiredApprovalsForSealConstruction
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
	}

	return nil
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
