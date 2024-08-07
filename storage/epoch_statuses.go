// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package storage

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

type EpochStatuses interface {

	// StoreTx stores a new epoch state in a DB transaction while going through the cache.
	StoreTx(blockID flow.Identifier, state *flow.EpochStatus) func(*transaction.Tx) error

	StorePebble(blockID flow.Identifier, state *flow.EpochStatus) func(PebbleReaderBatchWriter) error

	// ByBlockID will return the epoch status for the given block
	// Error returns:
	// * storage.ErrNotFound if EpochStatus for the block does not exist
	ByBlockID(flow.Identifier) (*flow.EpochStatus, error)
}
