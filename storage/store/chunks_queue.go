package store

import (
	"errors"
	"fmt"
	"sync"

	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

// ChunksQueue stores a queue of chunk locators that assigned to me to verify.
// Job consumers can read the locators as job from the queue by index.
// Chunk locators stored in this queue are unique.
type ChunksQueue struct {
	db      storage.DB
	storing *sync.Mutex
}

const JobQueueChunksQueue = "JobQueueChunksQueue"

// NewChunkQueue will initialize the underlying badger database of chunk locator queue.
func NewChunkQueue(db storage.DB) *ChunksQueue {
	return &ChunksQueue{
		db:      db,
		storing: &sync.Mutex{},
	}
}

// Init initializes chunk queue's latest index with the given default index.
func (q *ChunksQueue) Init(defaultIndex uint64) (bool, error) {
	_, err := q.LatestIndex()
	if errors.Is(err, storage.ErrNotFound) {
		err = q.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.InitJobLatestIndex(rw.Writer(), JobQueueChunksQueue, defaultIndex)
		})

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
func (q *ChunksQueue) StoreChunkLocator(locator *chunks.Locator) (bool, error) {
	// storing chunk locator requires reading the latest index and updating it,
	// so we need to lock the storing operation
	q.storing.Lock()
	defer q.storing.Unlock()

	// read the latest index
	var latest uint64
	err := operation.RetrieveJobLatestIndex(q.db.Reader(), JobQueueChunksQueue, &latest)
	if err != nil {
		return false, fmt.Errorf("failed to retrieve job index for chunk locator queue: %w", err)
	}

	// make sure the chunk locator is unique
	exists, err := operation.ExistChunkLocator(q.db.Reader(), locator.ID())
	if err != nil {
		return false, fmt.Errorf("failed to check chunk locator existence: %w", err)
	}

	// if the locator already exists, return false
	if exists {
		return false, nil
	}

	// insert to the next index
	next := latest + 1

	err = q.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		// make sure the chunk locator is unique
		err := operation.InsertChunkLocator(rw.Writer(), locator)
		if err != nil {
			return fmt.Errorf("failed to insert chunk locator: %w", err)
		}

		err = operation.InsertJobAtIndex(rw.Writer(), JobQueueChunksQueue, next, locator.ID())
		if err != nil {
			return fmt.Errorf("failed to set job index for chunk locator queue at index %v: %w", next, err)
		}

		// update the next index as the latest index
		err = operation.SetJobLatestIndex(rw.Writer(), JobQueueChunksQueue, next)
		if err != nil {
			return fmt.Errorf("failed to update latest index %v: %w", next, err)
		}

		return nil
	})

	if err != nil {
		return false, fmt.Errorf("failed to store chunk locator: %w", err)
	}
	return true, nil
}

// LatestIndex returns the index of the latest chunk locator stored in the queue.
func (q *ChunksQueue) LatestIndex() (uint64, error) {
	var latest uint64
	err := operation.RetrieveJobLatestIndex(q.db.Reader(), JobQueueChunksQueue, &latest)
	if err != nil {
		return 0, fmt.Errorf("could not retrieve latest index for chunks queue: %w", err)
	}
	return latest, nil
}

// AtIndex returns the chunk locator stored at the given index in the queue.
func (q *ChunksQueue) AtIndex(index uint64) (*chunks.Locator, error) {
	var locatorID flow.Identifier
	err := operation.RetrieveJobAtIndex(q.db.Reader(), JobQueueChunksQueue, index, &locatorID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve chunk locator in queue: %w", err)
	}

	var locator chunks.Locator
	err = operation.RetrieveChunkLocator(q.db.Reader(), locatorID, &locator)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve locator for chunk id %v: %w", locatorID, err)
	}

	return &locator, nil
}
