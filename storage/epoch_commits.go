// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package storage

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

type EpochCommits interface {

	// StoreTx allows us to store a new epoch commit in a DB transaction while updating the cache.
	StoreTx(commit *flow.EpochCommit) func(*badger.Txn) error

	// ByCommitID will return the EpochCommit event by its ID.
	ByID(flow.Identifier) (*flow.EpochCommit, error)
}
