package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

type EpochSetups interface {

	// BatchStore allows us to store a new epoch setup in a DB batch update while going through the cache.
	BatchStore(rw ReaderBatchWriter, setup *flow.EpochSetup) error

	// ByID will return the EpochSetup event by its ID.
	// Error returns:
	// * storage.ErrNotFound if no EpochSetup with the ID exists
	ByID(flow.Identifier) (*flow.EpochSetup, error)
}
