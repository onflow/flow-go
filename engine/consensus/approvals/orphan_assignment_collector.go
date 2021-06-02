package approvals

import (
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/consensus/approvals/tracker"
	"github.com/onflow/flow-go/model/flow"
)

type OrphanAssignmentCollector struct {
	resultID flow.Identifier
	blockID  flow.Identifier
}

func NewOrphanAssignmentCollector(resultID, blockID flow.Identifier) *OrphanAssignmentCollector {
	return &OrphanAssignmentCollector{
		resultID: resultID,
		blockID:  blockID,
	}
}

func (ac *OrphanAssignmentCollector) BlockID() flow.Identifier {
	return ac.blockID
}

func (ac *OrphanAssignmentCollector) ResultID() flow.Identifier {
	return ac.resultID
}

func (ac *OrphanAssignmentCollector) ProcessIncorporatedResult(*flow.IncorporatedResult) error {
	return engine.NewOutdatedInputErrorf("collector for %s is marked as non processable", ac.resultID)
}

func (ac *OrphanAssignmentCollector) ProcessApproval(approval *flow.ResultApproval) error {
	return engine.NewOutdatedInputErrorf("collector for %s is marked as non processable", approval.Body.ExecutionResultID)
}

func (ac *OrphanAssignmentCollector) CheckEmergencySealing(finalizedBlockHeight uint64) error {
	return engine.NewOutdatedInputErrorf("collector for %s is marked as non processable", ac.resultID)
}

func (ac *OrphanAssignmentCollector) RequestMissingApprovals(sealingTracker *tracker.SealingTracker, maxHeightForRequesting uint64) (int, error) {
	return 0, engine.NewOutdatedInputErrorf("collector for %s is marked as non processable", ac.resultID)
}
