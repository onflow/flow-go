// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package storage

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
)

// ViewRange is used to store the first and last views epochs in a lookup table.
type ViewRange struct {
	First uint64
	Last  uint64
}

type EpochSetups interface {

	// StoreTx allows us to store a new epoch setup in a DB transaction while going through the cache.
	StoreTx(*flow.EpochSetup) func(*badger.Txn) error

	// ByID will return the EpochSetup event by its ID.
	ByID(flow.Identifier) (*flow.EpochSetup, error)

	// CounterByView returns the epoch counter of a view.
	CounterByView(uint64) (uint64, error)
}
