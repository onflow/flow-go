package storage

import "github.com/onflow/flow-go/model/flow"

// LatestPersistedSealedResultReader tracks the most recently persisted sealed execution result processed
// by the Access ingestion engine and provides read-only access to it.
type LatestPersistedSealedResultReader interface {
	// Latest returns the ID and height of the latest persisted sealed result.
	Latest() (flow.Identifier, uint64)
}

// LatestPersistedSealedResult extends LatestPersistedSealedResultReader with mutating capabilities.
// It allows callers to atomically update the most recently persisted sealed execution result as part
// of a storage batch. Safe for concurrent access.
type LatestPersistedSealedResult interface {
	LatestPersistedSealedResultReader

	// BatchSet updates the latest persisted sealed result in a batch operation
	// The resultID and height are added to the provided batch, and the local data is updated only after
	// the batch is successfully committed.
	//
	// No errors are expected during normal operation,
	BatchSet(resultID flow.Identifier, height uint64, batch ReaderBatchWriter) error
}
