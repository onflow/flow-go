package approvals

import (
	"github.com/onflow/flow-go/engine/consensus"
	"github.com/onflow/flow-go/model/flow"
)

// OrphanAssignmentCollector is an AssignmentCollectorState with the fixed `ProcessingStatus` of `Orphaned`.
type OrphanAssignmentCollector struct {
	AssignmentCollectorBase
}

func NewOrphanAssignmentCollector(collectorBase AssignmentCollectorBase) AssignmentCollectorState {
	return &OrphanAssignmentCollector{
		AssignmentCollectorBase: collectorBase,
	}
}

func (oc *OrphanAssignmentCollector) ProcessingStatus() ProcessingStatus { return Orphaned }
func (oc *OrphanAssignmentCollector) CheckEmergencySealing(consensus.SealingObservation, uint64) error {
	return nil
}
func (oc *OrphanAssignmentCollector) RequestMissingApprovals(consensus.SealingObservation, uint64) (uint, error) {
	return 0, nil
}
func (oc *OrphanAssignmentCollector) ProcessIncorporatedResult(*flow.IncorporatedResult) error {
	return nil
}
func (oc *OrphanAssignmentCollector) ProcessApproval(*flow.ResultApproval) error {
	return nil
}
