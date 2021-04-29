package approvals

import (
	"sync"

	"github.com/onflow/flow-go/model/flow"
)

// ChunkApprovalCollector implements logic for checking chunks against assignments as
// well as accumulating signatures of already checked approvals.
type ChunkApprovalCollector struct {
	assignment                           map[flow.Identifier]struct{} // set of verifiers that were assigned to current chunk
	chunkApprovals                       *flow.SignatureCollector     // accumulator of signatures for current collector
	lock                                 sync.Mutex                   // lock to protect `chunkApprovals`
	requiredApprovalsForSealConstruction uint                         // number of approvals that are required for each chunk to be sealed
}

func NewChunkApprovalCollector(assignment map[flow.Identifier]struct{}, requiredApprovalsForSealConstruction uint) *ChunkApprovalCollector {
	return &ChunkApprovalCollector{
		assignment:                           assignment,
		chunkApprovals:                       flow.NewSignatureCollector(),
		lock:                                 sync.Mutex{},
		requiredApprovalsForSealConstruction: requiredApprovalsForSealConstruction,
	}
}

// ProcessApproval performs processing and bookkeeping of single approval
func (c *ChunkApprovalCollector) ProcessApproval(approval *flow.ResultApproval) (flow.AggregatedSignature, bool) {
	approverID := approval.Body.ApproverID
	if _, ok := c.assignment[approverID]; ok {
		c.lock.Lock()
		defer c.lock.Unlock()
		c.chunkApprovals.Add(approverID, approval.Body.AttestationSignature)
		if c.chunkApprovals.NumberSignatures() >= c.requiredApprovalsForSealConstruction {
			return c.chunkApprovals.ToAggregatedSignature(), true
		}
	}

	return flow.AggregatedSignature{}, false
}

// GetMissingSigners returns ids of approvers that are present in assignment but didn't provide approvals
func (c *ChunkApprovalCollector) GetMissingSigners() flow.IdentifierList {
	// provide capacity for worst-case
	result := make(flow.IdentifierList, 0, len(c.assignment))
	c.lock.Lock()
	for id := range c.assignment {
		if _, hasSigned := c.chunkApprovals.BySigner(id); !hasSigned {
			result = append(result, id)
		}
	}
	c.lock.Unlock()

	return result
}
