// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package module

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Finalizer represents a wrapper around the temporary state (memory pools) and
// the protocol state, which introduces finalized blocks and frees resources
// for pending entities that are no longer needed.
type Finalizer interface {

	// MakeFinal will declare a block and all of its children as finalized, which
	// makes it an immutable part of the blockchain, and cleans up pending
	// entities that were included in the block or its ancestors.
	MakeFinal(blockID flow.Identifier) error
}
