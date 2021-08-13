package module

import "github.com/onflow/flow-go/model/flow"

// SealValidator checks seals with respect to current protocol state.
// Accepts `candidate` block with seals
// that needs to be verified for protocol state validity.
// Returns the following values:
// * last seal in `candidate` block - in case of success
// * engine.InvalidInputError - in case if `candidate` block violates protocol state.
// * exception in case of any other error, usually this is not expected.
// PREREQUISITE:
// The SealValidator can only process blocks which are attached to the main chain
// (without any missing ancestors). This is the case because:
//  * the Seal validator walks the chain backwards and requires the relevant ancestors to be known and validated
//  * the storage.Seals only holds seals for block that are attached to the main chain.
type SealValidator interface {
	Validate(candidate *flow.Block) (*flow.Seal, error)
}
