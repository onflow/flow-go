package module

import "github.com/onflow/flow-go/model/flow"

// SealValidator is an interface which is used for validating
// seals with respect to current protocol state. Except seal to check,
// user needs to provide a `ExecutionResult` against which seal has to be validated.
// Returns the following errors:
// * nil - in case of success
// * exception in case of any other error, usually this is not expected.
type SealValidator interface {
	Validate(block *flow.Block) (*flow.Seal, error)
}
