// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package storage

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

type EpochSetups interface {

	// StoreTx allows us to store a new epoch setup in a DB transaction while going through the cache.
	StoreTx(*flow.EpochSetup) func(*badger.Txn) error

	StoreTxn(*flow.EpochSetup) func(*transaction.Tx) error

	// ByID will return the EpochSetup event by its ID.
	ByID(flow.Identifier) (*flow.EpochSetup, error)
}
