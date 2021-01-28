package module

import "github.com/onflow/flow-go/model/flow"

// ApprovalValidator is an interface which is used for validating
// result approvals received from verification nodes with respect
//	to current protocol state.
// Returns the following errors:
// * nil - in case of success
// * sentinel engine.InvalidInputError when approval is not valid due to
// conflicting protocol state.
// * exception in case of any other error, usually this is not expected.
type ApprovalValidator interface {
	Validate(approval *flow.ResultApproval) error
}
