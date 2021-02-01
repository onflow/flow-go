package badger

import (
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
)

// ChunksQueue stores a queue of chunks that assigned to me to be verified.
// Job consumers can read the chunk as job from the queue by index
// Chunks stored in this queue are unique
type ChunksQueue struct {
	db *badger.DB
}

const JobQueueChunksQueue = "JobQueueChunksQueue"

// NewChunksQueue will initialize the index of the latest chunk stored with
// the given default index if it was never initialized
func NewChunksQueue(db *badger.DB) *ChunksQueue {
	return &ChunksQueue{
		db: db,
	}
}

// Init initial chunks queue's latest index witht the given default index
func (q *ChunksQueue) Init(defaultIndex int64) (bool, error) {
	_, err := q.LatestIndex()
	if errors.Is(err, storage.ErrNotFound) {
		err = q.db.Update(operation.InitJobLatestIndex(JobQueueChunksQueue, defaultIndex))
		if err != nil {
			return false, fmt.Errorf("could not init chunks queue with default index %v: %w", defaultIndex, err)
		}
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("could not get latest index: %w", err)
	}

	return false, nil
}

// StoreChunk stores a new chunk that assigned to me to the job queue
// true will be returned, if the chunk was a new chunk
// false will be returned, if the chunk was a duplicate
func (q *ChunksQueue) StoreChunk(chunk *flow.Chunk) (bool, error) {
	err := operation.RetryOnConflict(q.db.Update, func(tx *badger.Txn) error {
		// make sure the chunk is unique
		err := operation.InsertChunk(chunk)(tx)
		if err != nil {
			return fmt.Errorf("failed to insert chunk: %w", err)
		}

		// read the latest index
		var latest int64
		err = operation.RetrieveJobLatestIndex(JobQueueChunksQueue, &latest)(tx)
		if err != nil {
			return fmt.Errorf("failed to retrieve job index for chunks queue: %w", err)
		}

		// insert to the next index
		next := latest + 1
		err = operation.InsertJobAtIndex(JobQueueChunksQueue, next, chunk.ID())(tx)
		if err != nil {
			return fmt.Errorf("failed to set job index for chunks queue at index %v: %w", next, err)
		}

		// update the next index as the latest index
		err = operation.SetJobLatestIndex(JobQueueChunksQueue, next)(tx)
		if err != nil {
			return fmt.Errorf("failed to update latest index %v: %w", next, err)
		}

		return nil
	})

	// was trying to store a duplicate chunk
	if errors.Is(err, storage.ErrAlreadyExists) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to store chunk: %w", err)
	}
	return true, nil
}

// LatestIndex returns the index of the latest chunk stored
// in the queue
func (q *ChunksQueue) LatestIndex() (int64, error) {
	var latest int64
	err := q.db.View(operation.RetrieveJobLatestIndex(JobQueueChunksQueue, &latest))
	if err != nil {
		return 0, fmt.Errorf("could not retrieve latest index for chunks queue: %w", err)
	}
	return latest, nil
}

// AtIndex returns the chunk stored at the given index in the
// queue
func (q *ChunksQueue) AtIndex(index int64) (*flow.Chunk, error) {
	var chunkID flow.Identifier
	err := q.db.View(operation.RetrieveJobAtIndex(JobQueueChunksQueue, index, &chunkID))
	if err != nil {
		return nil, fmt.Errorf("could not retrieve chunk in chunks queue: %w", err)
	}

	var chunk flow.Chunk
	err = q.db.View(operation.RetrieveChunk(chunkID, &chunk))
	if err != nil {
		return nil, fmt.Errorf("could not retrieve chunk with id %v: %w", chunkID, err)
	}

	return &chunk, nil
}
