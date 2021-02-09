package module

import "github.com/onflow/flow-go/model/flow"

// ApprovalValidator is used for validating
// result approvals received from verification nodes with respect
//	to current protocol state.
// Returns the following:
// * nil - in case of success
// * sentinel engine.InvalidInputError when approval is invalid 
// * sentinel engine.OutdatedInputError if the corresponding block has a finalized seal
// * exception in case of any other error, usually this is not expected.
type ApprovalValidator interface {
	Validate(approval *flow.ResultApproval) error
}
