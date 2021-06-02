package approvals

import (
	"fmt"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/consensus/approvals/tracker"
	"github.com/onflow/flow-go/model/flow"
)

type CachingAssignmentCollector struct {
	resultID       flow.Identifier
	blockID        flow.Identifier
	approvalsCache *Cache // in-memory cache of approvals (not-verified)
}

func (ac *CachingAssignmentCollector) BlockID() flow.Identifier {
	return ac.blockID
}

func (ac *CachingAssignmentCollector) ResultID() flow.Identifier {
	return ac.resultID
}

func (ac *CachingAssignmentCollector) ProcessIncorporatedResult(*flow.IncorporatedResult) error {
	return nil
}

func (ac *CachingAssignmentCollector) ProcessApproval(approval *flow.ResultApproval) error {
	// check that approval is for the expected result to reject incompatible inputs
	if approval.Body.ExecutionResultID != ac.resultID {
		return fmt.Errorf("this CachingAssignmentCollector processes only approvals for result (%x) but got an approval for (%x)", ac.resultID, approval.Body.ExecutionResultID)
	}

	// approval has to refer same block as execution result
	if approval.Body.BlockID != ac.BlockID() {
		return engine.NewInvalidInputErrorf("result approval for invalid block, expected (%x) vs (%x)",
			ac.BlockID(), approval.Body.BlockID)
	}

	// we have this approval cached already, no need to process it again
	approvalCacheID := approval.Body.PartialID()
	if cached := ac.approvalsCache.Get(approvalCacheID); cached != nil {
		return nil
	}

	ac.approvalsCache.Put(approval)

	return nil
}

func (ac *CachingAssignmentCollector) CheckEmergencySealing(uint64) error {
	return nil
}

func (ac *CachingAssignmentCollector) RequestMissingApprovals(*tracker.SealingTracker, uint64) (int, error) {
	return 0, nil
}
