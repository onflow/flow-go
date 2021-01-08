package module

import "github.com/onflow/flow-go/model/flow"

// SealValidator checks seals with respect to current protocol state.
// Accepts `candidate` block with seals
// that needs to be verified for protocol state validity.
// Returns the following values:
// * last seal in `candidate` block - in case of success
// * engine.InvalidInputError - in case if `candidate` block violates protocol state.
// * exception in case of any other error, usually this is not expected.
type SealValidator interface {
	Validate(candidate *flow.Block) (*flow.Seal, error)
}
