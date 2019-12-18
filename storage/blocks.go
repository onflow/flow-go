// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package storage

import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

// Error indicating that requested block was not found
// and no other errors occurred during querying (simply data was not there)
type BlockNotFoundErr struct {
	hash   crypto.Hash
	number uint64
}

func (err BlockNotFoundErr) Error() string {
	if err.hash != nil {
		return fmt.Sprintf("block with hash %v not found", err.hash)
	} else {
		return fmt.Sprintf("block number %v not found", err.number)
	}
}

func BlockNotFoundErrWithHash(hash crypto.Hash) error {
	return BlockNotFoundErr{
		hash: hash,
	}
}

func BlockNotFoundErrWithNumber(number uint64) error {
	return BlockNotFoundErr{
		number: number,
	}
}

// Blocks represents persistent storage for blocks.
type Blocks interface {
	// ByHash returns the block with the given hash. It is available for
	// finalized and ambiguous blocks.
	ByHash(hash crypto.Hash) (*flow.Block, Error)

	// ByNumber returns the block with the given number. It is only available
	// for finalized blocks.
	ByNumber(number uint64) (*flow.Block, Error)

	Save(*flow.Block) Error
}
