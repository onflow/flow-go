package approvals

import (
	"fmt"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
)

type ApprovalCollector struct {
	incorporatedBlockID           flow.Identifier
	chunkCollectors               []*ChunkApprovalCollector
	assignmentAuthorizedVerifiers map[flow.Identifier]struct{}
}

func (c *ApprovalCollector) ProcessApproval(approval *flow.ResultApproval) error {
	chunkIndex := approval.Body.ChunkIndex
	if chunkIndex >= uint64(len(c.chunkCollectors)) {
		return engine.NewInvalidInputErrorf("approval collector chunk index out of range: %v", chunkIndex)
	}
	collector := c.chunkCollectors[chunkIndex]
	err := collector.ProcessApproval(approval)
	if err != nil {
		return fmt.Errorf("could not process approval: %w", err)
	}
	return nil
}

func NewApprovalCollector(result *flow.IncorporatedResult, assignment *chunks.Assignment, authorizedVerifiers map[flow.Identifier]struct{}) *ApprovalCollector {
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
		incorporatedBlockID:           result.IncorporatedBlockID,
		chunkCollectors:               chunkCollectors,
		assignmentAuthorizedVerifiers: authorizedVerifiers,
	}
}
