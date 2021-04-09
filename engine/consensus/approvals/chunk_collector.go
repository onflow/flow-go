package approvals

import (
	"sync"

	"github.com/onflow/flow-go/model/flow"
)

type ChunkProcessingStatus struct {
	numberOfApprovals uint
	approvalProcessed bool
}

// ChunkApprovalCollector implements logic for checking chunks against assignments as
// well as accumulating signatures of already checked approvals.
type ChunkApprovalCollector struct {
	assignment          map[flow.Identifier]struct{} // set of verifiers that were assigned to current chunk
	authorizedVerifiers map[flow.Identifier]struct{} // set of authorized verifiers that are authorized for current incorporated result
	chunkApprovals      *flow.SignatureCollector     // accumulator of signatures for current collector
	lock                sync.Mutex                   // lock to protect `chunkApprovals`
}

func NewChunkApprovalCollector(assignment map[flow.Identifier]struct{},
	authorizedVerifiers map[flow.Identifier]struct{}) *ChunkApprovalCollector {
	return &ChunkApprovalCollector{
		assignment:          assignment,
		authorizedVerifiers: authorizedVerifiers,
		chunkApprovals:      flow.NewSignatureCollector(),
		lock:                sync.Mutex{},
	}
}

// ProcessApproval performs processing and bookkeeping of single approval
func (c *ChunkApprovalCollector) ProcessApproval(approval *flow.ResultApproval) ChunkProcessingStatus {
	status := ChunkProcessingStatus{
		numberOfApprovals: 0,
		approvalProcessed: false,
	}

	approverID := approval.Body.ApproverID
	if _, ok := c.assignment[approverID]; !ok {
		return status
	}
	if _, ok := c.authorizedVerifiers[approverID]; !ok {
		return status
	}

	c.lock.Lock()
	defer c.lock.Unlock()
	c.chunkApprovals.Add(approverID, approval.Body.AttestationSignature)
	status.approvalProcessed = true
	status.numberOfApprovals = c.chunkApprovals.NumberSignatures()

	return status
}

func (c *ChunkApprovalCollector) GetAggregatedSignature() flow.AggregatedSignature {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.chunkApprovals.ToAggregatedSignature()
}
