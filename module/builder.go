package module

import (
	"github.com/onflow/flow-go/model/flow"
)

// Builder represents an abstracted block construction module that can be used
// in more than one consensus algorithm. The resulting block is consistent
// within itself and can be wrapped with additional consensus information such
// as QCs.
type Builder interface {

	// BuildOn generates a new payload that is valid with respect to the parent
	// being built upon, with the view being provided by the consensus algorithm.
	// The builder stores the block and validates it against the protocol state
	// before returning it. The specified parent block must exist in the protocol state.
	//
	// NOTE: Since the block is stored within Builder, HotStuff MUST propose the
	// block once BuildOn successfully returns.
	//
	// # Errors
	// This function does not produce any expected errors.
	// However, it will pass through all errors returned by `setter` and `sign`.
	// Callers must be aware of possible error returns from the `setter` and `sign` arguments they provide,
	// and handle them accordingly when handling errors returned from BuildOn.
	BuildOn(parentID flow.Identifier, setter func(*flow.Header) error, sign func(*flow.Header) ([]byte, error)) (*flow.Proposal, error)
}
