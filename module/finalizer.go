package module

import (
	"github.com/onflow/flow-go/model/flow"
)

// Finalizer is used by the consensus algorithm to inform other components for (such
// as the protocol state) about finalization of blocks.
//
// Since we have two different protocol states: one for the main consensus,
// the other for the collection cluster consensus, the Finalizer interface
// allows the two different protocol states to provide different implementations
// for updating its state when a block has been finalized.
//
// Updating the protocol state should always succeed when the data is consistent.
// However, in case the protocol state is corrupted, error should be returned and
// the consensus algorithm should halt. So the error returned from MakeFinal is
// for the protocol state to report exceptions.
type Finalizer interface {

	// MakeFinal will declare a block and all of its ancestors as finalized, which
	// makes it an immutable part of the blockchain. Returning an error indicates
	// some fatal condition and will cause the finalization logic to terminate.
	MakeFinal(blockID flow.Identifier) error
}
