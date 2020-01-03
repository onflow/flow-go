// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package storage

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

// Blocks represents persistent storage for blocks.
type Blocks interface {
	// ByHash returns the block with the given hash. It is available for
	// finalized and ambiguous blocks.
	ByHash(hash crypto.Hash) (*flow.Block, error)

	// ByNumber returns the block with the given number. It is only available
	// for finalized blocks.
	ByNumber(number uint64) (*flow.Block, error)

	// Save stores the  block. If the exactly same block is already in a storage, return successfully
	Save(*flow.Block) error
}
