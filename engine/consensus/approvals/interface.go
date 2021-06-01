package approvals

import (
	"github.com/onflow/flow-go/engine/consensus/approvals/tracker"
	"github.com/onflow/flow-go/model/flow"
)

type AssignmentCollector interface {
	BlockID() flow.Identifier
	ResultID() flow.Identifier

	ProcessIncorporatedResult(incorporatedResult *flow.IncorporatedResult) error
	ProcessApproval(approval *flow.ResultApproval) error

	CheckEmergencySealing(finalizedBlockHeight uint64) error
	RequestMissingApprovals(sealingTracker *tracker.SealingTracker, maxHeightForRequesting uint64) (int, error)
}
