package approvals

import (
	"github.com/onflow/flow-go/engine/consensus"
	"github.com/onflow/flow-go/model/flow"
)

type OrphanAssignmentCollector struct {
	assignmentCollectorBase
}

func NewOrphanAssignmentCollector(collectorBase assignmentCollectorBase) AssignmentCollectorState {
	return &OrphanAssignmentCollector{
		assignmentCollectorBase: collectorBase,
	}
}

func (oc *OrphanAssignmentCollector) ProcessingStatus() ProcessingStatus { return Orphaned }
func (oc *OrphanAssignmentCollector) CheckEmergencySealing(uint64, consensus.SealingObservation) error {
	return nil
}
func (oc *OrphanAssignmentCollector) RequestMissingApprovals(uint64, consensus.SealingObservation) (uint, error) {
	return 0, nil
}
func (oc *OrphanAssignmentCollector) ProcessIncorporatedResult(*flow.IncorporatedResult) error {
	return nil
}
func (oc *OrphanAssignmentCollector) ProcessApproval(*flow.ResultApproval) error {
	return nil
}
