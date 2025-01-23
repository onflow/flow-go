package module

import (
	"github.com/onflow/flow-go/model/flow"
)

// SetHeaderHotstuffFieldsFunc is a function which sets the flow.Header fields
// determined by the HotStuff module, instead of the Protocol State.
// No errors are expected during normal operation.
type SetHeaderHotstuffFieldsFunc func(header *flow.Header) error

// SignBlockHeaderFunc is a function which produces a signature over a complete
// block proposal produced by this node, and sets the corresponding ProposerSig
// fields in the header argument.
// Error Returns:
//   - model.NoVoteError if it is not safe for us to vote (our proposal includes our vote)
//     for this view. This can happen if we have already proposed or timed out this view.
//   - generic error in case of unexpected failure
type SignBlockHeaderFunc func(header *flow.Header) error

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
	// This function does not produce any expected error types. However, it will pass
	// through errors returned by `setter` and `sign`. Callers must be aware of
	// possible error returns from these functional parameters, and handle them accordingly
	// when handling errors returned from BuildOn.
	BuildOn(parentID flow.Identifier, setter func(*flow.Header) error, sign func(*flow.Header) error) (*flow.Header, error)
}
