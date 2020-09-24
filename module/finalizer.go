// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package module

import (
	"github.com/onflow/flow-go/model/flow"
)

// Finalizer is used by the consensus' finalization logic to inform
// other components in the node about a block being finalized.
type Finalizer interface {

	// MakeValid will mark a block as having passed the consensus algorithm's
	// internal validation.
	MakeValid(blockID flow.Identifier) error

	// MakeFinal will declare a block and all of its ancestors as finalized, which
	// makes it an immutable part of the blockchain. Returning an error indicates
	// some fatal condition and will cause the finalization logic to terminate.
	MakeFinal(blockID flow.Identifier) error
}
