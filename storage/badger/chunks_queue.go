package badger

import (
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
)

// ChunkQueue stores a queue of chunk locators that assigned to me to verify.
// Job consumers can read the locators as job from the queue by index.
// Chunk locators stored in this queue are unique.
type ChunkQueue struct {
	db *badger.DB
}

const JobQueueChunkQueue = "JobQueueChunkQueue"

// NewChunkQueue will initialize the underlying badger database of chunk locator queue.
func NewChunkQueue(db *badger.DB) *ChunkQueue {
	return &ChunkQueue{
		db: db,
	}
}

// Init initial chunk locator queue's latest index with the given default index.
func (q *ChunkQueue) Init(defaultIndex int64) (bool, error) {
	_, err := q.LatestIndex()
	if errors.Is(err, storage.ErrNotFound) {
		err = q.db.Update(operation.InitJobLatestIndex(JobQueueChunkQueue, defaultIndex))
		if err != nil {
			return false, fmt.Errorf("could not init chunk locator queue with default index %v: %w", defaultIndex, err)
		}
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("could not get latest index: %w", err)
	}

	return false, nil
}

// StoreChunkLocator stores a new chunk locator that assigned to me to the job queue.
// A true will be returned, if the locator was new.
// A false will be returned, if the locator was duplicate.
func (q *ChunkQueue) StoreChunkLocator(locator *chunks.Locator) (bool, error) {
	err := operation.RetryOnConflict(q.db.Update, func(tx *badger.Txn) error {
		// make sure the chunk locator is unique
		err := operation.InsertChunkLocator(locator)(tx)
		if err != nil {
			return fmt.Errorf("failed to insert chunk locator: %w", err)
		}

		// read the latest index
		var latest int64
		err = operation.RetrieveJobLatestIndex(JobQueueChunkQueue, &latest)(tx)
		if err != nil {
			return fmt.Errorf("failed to retrieve job index for chunk locator queue: %w", err)
		}

		// insert to the next index
		next := latest + 1
		err = operation.InsertJobAtIndex(JobQueueChunkQueue, next, locator.ID())(tx)
		if err != nil {
			return fmt.Errorf("failed to set job index for chunk locator queue at index %v: %w", next, err)
		}

		// update the next index as the latest index
		err = operation.SetJobLatestIndex(JobQueueChunkQueue, next)(tx)
		if err != nil {
			return fmt.Errorf("failed to update latest index %v: %w", next, err)
		}

		return nil
	})

	// was trying to store a duplicate locator
	if errors.Is(err, storage.ErrAlreadyExists) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to store chunk locator: %w", err)
	}
	return true, nil
}

// LatestIndex returns the index of the latest chunk locator stored in the queue.
func (q *ChunkQueue) LatestIndex() (int64, error) {
	var latest int64
	err := q.db.View(operation.RetrieveJobLatestIndex(JobQueueChunkQueue, &latest))
	if err != nil {
		return 0, fmt.Errorf("could not retrieve latest index for chunks queue: %w", err)
	}
	return latest, nil
}

// AtIndex returns the chunk locator stored at the given index in the queue.
func (q *ChunkQueue) AtIndex(index int64) (*chunks.Locator, error) {
	var chunkID flow.Identifier
	err := q.db.View(operation.RetrieveJobAtIndex(JobQueueChunkQueue, index, &chunkID))
	if err != nil {
		return nil, fmt.Errorf("could not retrieve chunk locator in queue: %w", err)
	}

	var locator chunks.Locator
	err = q.db.View(operation.RetrieveChunkLocator(chunkID, &locator))
	if err != nil {
		return nil, fmt.Errorf("could not retrieve locator for chunk id %v: %w", chunkID, err)
	}

	return &locator, nil
}
