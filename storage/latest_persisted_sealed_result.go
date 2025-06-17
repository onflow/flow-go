package storage

import "github.com/onflow/flow-go/model/flow"

type LatestPersistedSealedResult interface {
	// Latest returns the ID and height of the latest persisted sealed result.
	Latest() (flow.Identifier, uint64)

	// BatchSet updates the latest persisted sealed result in a batch operation
	// The resultID and height are added to the provided batch, and the local data is updated only after
	// the batch is successfully committed.
	//
	// No errors are expected during normal operation,
	BatchSet(resultID flow.Identifier, height uint64, batch ReaderBatchWriter) error
}
