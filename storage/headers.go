// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package storage

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Headers represents persistent storage for blocks.
type Headers interface {

	// Store will store a header.
	Store(header *flow.Header) error

	// ByBlockID returns the header with the given ID. It is available for
	// finalized and ambiguous blocks.
	ByBlockID(blockID flow.Identifier) (*flow.Header, error)

	// ByNumber returns the block with the given number. It is only available
	// for finalized blocks.
	ByNumber(number uint64) (*flow.Header, error)
}
