// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package storage

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

type EpochStates interface {

	// StoreTx stores a new epoch state in a DB transaction while going through the cache.
	StoreTx(blockID flow.Identifier, state *flow.EpochState) func(*badger.Txn) error

	// ByBlockID will return the EpochSetup for the given block
	ByBlockID(flow.Identifier) (*flow.EpochState, error)
}
