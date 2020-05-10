package storage

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

type Index interface {

	// Store stores the index for a block payload.
	Store(blockID flow.Identifier, index flow.Index) error

	// ByBlockID retrieves the index for a block payload.
	ByBlockID(blockID flow.Identifier) (flow.Index, error)
}
