package approvals

import (
	"sync"

	"github.com/onflow/flow-go/model/flow"
)

// ChunkProcessingStatus is a helper structure which is used to report
// chunk processing status to caller
type ChunkProcessingStatus struct {
	numberOfApprovals uint
	approvalProcessed bool
}

// ChunkApprovalCollector implements logic for checking chunks against assignments as
// well as accumulating signatures of already checked approvals.
type ChunkApprovalCollector struct {
	assignment     map[flow.Identifier]struct{} // set of verifiers that were assigned to current chunk
	chunkApprovals *flow.SignatureCollector     // accumulator of signatures for current collector
	lock           sync.Mutex                   // lock to protect `chunkApprovals`
}

func NewChunkApprovalCollector(assignment map[flow.Identifier]struct{}) *ChunkApprovalCollector {
	return &ChunkApprovalCollector{
		assignment:     assignment,
		chunkApprovals: flow.NewSignatureCollector(),
		lock:           sync.Mutex{},
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

	c.lock.Lock()
	defer c.lock.Unlock()
	c.chunkApprovals.Add(approverID, approval.Body.AttestationSignature)
	status.approvalProcessed = true
	status.numberOfApprovals = c.chunkApprovals.NumberSignatures()

	return status
}

// GetAggregatedSignature returns an aggregated signature for this chunk
func (c *ChunkApprovalCollector) GetAggregatedSignature() flow.AggregatedSignature {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.chunkApprovals.ToAggregatedSignature()
}

// GetMissingSigners returns ids of approvers that are present in assignment but didn't provide approvals
func (c *ChunkApprovalCollector) GetMissingSigners() flow.IdentifierList {
   // provide capacity for worst-case
   result := make(flow.IdentifierList, 0, len(c.assignment))
	aggregatedSig := c.GetAggregatedSignature()
	c.lock.Lock()
	for id := range c.assignment {
		if  _, hasSigned := c.chunkApprovals.BySigner(id); !hasSigned {
			result = append(result, id)
		}
	}
	c.lock.Unlock()

	return result
}
