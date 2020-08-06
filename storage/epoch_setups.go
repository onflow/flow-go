// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package storage

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

type EpochSetups interface {

	// StoreTx allows us to store a new epoch setup in a DB transaction while going through the cache.
	StoreTx(setup *flow.EpochSetup) func(*badger.Txn) error

	// ByCounter will return the setup for an epoch by counter.
	ByCounter(counter uint64) (*flow.EpochSetup, error)
}
