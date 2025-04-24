package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

type EpochCommits interface {

	// BatchStore allows us to store a new epoch commit in a DB batch update while updating the cache.
	BatchStore(rw ReaderBatchWriter, commit *flow.EpochCommit) error

	// ByID will return the EpochCommit event by its ID.
	// Error returns:
	// * storage.ErrNotFound if no EpochCommit with the ID exists
	ByID(flow.Identifier) (*flow.EpochCommit, error)
}
