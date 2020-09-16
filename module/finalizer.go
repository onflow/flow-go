// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package module

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Finalizer is used by the consensus algorithm to inform protocol state for
// about certain events.
//
// Since we have two different protocol states: one for the main consensus,
// the other for the collection cluster consensus, the Finalizer interface
// allows the two different protocol states to provide different implementations
// for updating its state when a block has been validated or finalized.
//
// Why MakeValid and MakeFinal need to return an error?
// Updating the protocol state should always succeed when the data is consistent.
// However, in case the protocol state is corrupted, error should be returned and
// the consensus algorithm should halt. So the error returned from MakeValid and
// MakeFinal is for the protocol state to report exceptions.
type Finalizer interface {

	// MakeValid will mark a block as having passed the consensus algorithm's
	// internal validation.
	MakeValid(blockID flow.Identifier) error

	// MakeFinal will declare a block and all of its ancestors as finalized, which
	// makes it an immutable part of the blockchain. Returning an error indicates
	// some fatal condition and will cause the finalization logic to terminate.
	MakeFinal(blockID flow.Identifier) error
}
