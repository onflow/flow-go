package approvals

import (
	"sync"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
)

type SealingRecord struct {
	// aggregated signature for every chunk with sufficient number of approvals
	AggregatedSignatures []flow.AggregatedSignature

	// SufficientApprovalsForSealing: True iff all chunks in the result have
	// sufficient approvals
	SufficientApprovalsForSealing bool
}

func NewRecordWithSufficientApprovals(signatures map[uint64]flow.AggregatedSignature) *SealingRecord {
	sigs := make([]flow.AggregatedSignature, len(signatures))
	for chunkIndex, sig := range signatures {
		sigs[chunkIndex] = sig
	}
	return &SealingRecord{
		AggregatedSignatures:          sigs,
		SufficientApprovalsForSealing: true,
	}
}

func NewRecordWithInSufficientApprovals() *SealingRecord {
	return &SealingRecord{
		AggregatedSignatures:          nil,
		SufficientApprovalsForSealing: false,
	}
}

type ApprovalCollector struct {
	incorporatedBlockID                  flow.Identifier
	numberOfChunks                       int
	chunkCollectors                      []*ChunkApprovalCollector
	assignmentAuthorizedVerifiers        map[flow.Identifier]struct{}
	requiredApprovalsForSealConstruction uint                                // min number of approvals required for constructing a candidate seal
	aggregatedSignatures                 map[uint64]flow.AggregatedSignature // aggregated signature for each chunk
	lock                                 sync.RWMutex                        // lock for modifying aggregatedSignatures
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
		numberOfChunks:                       result.Result.Chunks.Len(),
		chunkCollectors:                      chunkCollectors,
		assignmentAuthorizedVerifiers:        authorizedVerifiers,
		requiredApprovalsForSealConstruction: requiredApprovalsForSealConstruction,
		aggregatedSignatures:                 make(map[uint64]flow.AggregatedSignature, result.Result.Chunks.Len()),
	}
}

func (c *ApprovalCollector) hasEnoughApprovals(chunkIndex uint64) bool {
	c.lock.RLock()
	defer c.lock.RUnlock()
	_, found := c.aggregatedSignatures[chunkIndex]
	return found
}

func (c *ApprovalCollector) ProcessApproval(approval *flow.ResultApproval) (*SealingRecord, error) {
	chunkIndex := approval.Body.ChunkIndex
	if chunkIndex >= uint64(len(c.chunkCollectors)) {
		return nil, engine.NewInvalidInputErrorf("approval collector chunk index out of range: %v", chunkIndex)
	}
	// there is no need to process approval if we have already enough info for sealing
	if c.hasEnoughApprovals(chunkIndex) {
		return NewRecordWithInSufficientApprovals(), nil
	}

	collector := c.chunkCollectors[chunkIndex]
	status := collector.ProcessApproval(approval)
	if status.numberOfApprovals >= c.requiredApprovalsForSealConstruction {
		c.collectAggregatedSignature(chunkIndex, collector)
		return c.checkSealingStatus(), nil
	}

	return NewRecordWithInSufficientApprovals(), nil
}

func (c *ApprovalCollector) checkSealingStatus() *SealingRecord {
	c.lock.RLock()
	defer c.lock.RUnlock()

	if len(c.aggregatedSignatures) == c.numberOfChunks {
		return NewRecordWithSufficientApprovals(c.aggregatedSignatures)
	}

	return NewRecordWithInSufficientApprovals()
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
