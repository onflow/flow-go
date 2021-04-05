package approvals

import (
	"sync"

	"github.com/onflow/flow-go/model/flow"
)

type ChunkProcessingStatus struct {
	numberOfApprovals *uint
	approvalProcessed bool
}

// ChunkApprovalCollector implements logic for checking chunks against assignments as
// well as accumulating signatures of already checked approvals.
type ChunkApprovalCollector struct {
	assignment          map[flow.Identifier]struct{}
	authorizedVerifiers map[flow.Identifier]struct{}
	chunkApprovals      *flow.SignatureCollector
	lock                sync.Mutex // lock to protect `chunkApprovals`
}

// ProcessApproval performs processing and bookkeeping of single approval
func (c *ChunkApprovalCollector) ProcessApproval(approval *flow.ResultApproval) ChunkProcessingStatus {
	status := ChunkProcessingStatus{
		numberOfApprovals: nil,
		approvalProcessed: false,
	}

	approverID := approval.Body.ApproverID
	if _, ok := c.assignment[approverID]; !ok {
		return status
	}
	if _, ok := c.authorizedVerifiers[approverID]; !ok {
		return status
	}

	var numbersOfApprovals uint
	c.lock.Lock()
	c.chunkApprovals.Add(approverID, approval.Body.AttestationSignature)
	numbersOfApprovals = c.chunkApprovals.NumberSignatures()
	c.lock.Unlock()

	status.approvalProcessed = true
	status.numberOfApprovals = &numbersOfApprovals

	return status
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
