// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package storage

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

type EpochStatuses interface {

	// StoreTx stores a new epoch state in a DB transaction while going through the cache.
	StoreTx(blockID flow.Identifier, state *flow.EpochStatus) func(*badger.Txn) error

	StoreTxn(blockID flow.Identifier, state *flow.EpochStatus) func(*transaction.Tx) error

	// ByBlockID will return the epoch status for the given block
	ByBlockID(flow.Identifier) (*flow.EpochStatus, error)
}
