package approvals

import (
	"fmt"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
)

type ApprovalCollector struct {
	incorporatedBlockID           flow.Identifier
	chunkCollectors               map[uint64]*ChunkApprovalCollector
	assignmentAuthorizedVerifiers map[flow.Identifier]struct{}
}

func (c *ApprovalCollector) ProcessApproval(approval *flow.ResultApproval) error {
	chunkIndex := approval.Body.ChunkIndex
	collector, ok := c.chunkCollectors[chunkIndex]
	if !ok {
		return engine.NewInvalidInputErrorf("approval collector chunk index out of range: %v", chunkIndex)
	}

	err := collector.ProcessApproval(approval)
	if err != nil {
		return fmt.Errorf("could not process approval: %w", err)
	}
	return nil
}

func NewApprovalCollector(assignment chunks.Assignment, ...) *ApprovalCollector {

}
