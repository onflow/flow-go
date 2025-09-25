package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

type Index interface {

	// ByBlockID retrieves the index for a block payload.
	// Error returns:
	//  - ErrNotFound if no block header with the given ID exists
	ByBlockID(blockID flow.Identifier) (*flow.Index, error)
}
