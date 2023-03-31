// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

type EpochCommits[tx Transaction] interface {

	// StoreTx allows us to store a new epoch commit in a DB transaction while updating the cache.
	StoreTx(commit *flow.EpochCommit) func(TransactionContext[tx]) error

	// ByID will return the EpochCommit event by its ID.
	// Error returns:
	// * storage.ErrNotFound if no EpochCommit with the ID exists
	ByID(flow.Identifier) (*flow.EpochCommit, error)
}
