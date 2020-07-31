// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package storage

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/epoch"
)

type EpochCommits interface {

	// StoreTx allows us to store a new epoch commit in a DB transaction while updating the cache.
	StoreTx(commit *epoch.Commit) func(*badger.Txn) error

	// CommitByCounter will return the commit for an epoch by counter.
	ByCounter(counter uint64) (*epoch.Commit, error)
}
