package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
)

// ChunksQueue stores a queue of chunks that assigned to me to be verified.
// Job consumers can read the chunk as job from the queue by index
// Chunks stored in this queue are unique
type ChunksQueue struct {
	db *badger.DB
}

// NewChunksQueue will initialize the index of the latest chunk stored with
// init index
func NewChunksQueue(db *badger.DB) *ChunksQueue {
	return &ChunksQueue{
		db: db,
	}
}

// StoreChunk stores a new chunk that assigned to me to the job queue
// true will be returned, if the chunk was a new chunk
// false will be returned, if the chunk was a duplicate
func StoreChunk(chunk *flow.Chunk) (bool, error) {
	return false, fmt.Errorf("to be implemented")
}

// LatestIndex returns the index of the latest chunk stored
// in the queue
func LatestIndex() (int64, error) {
	return 0, fmt.Errorf("to be implemented")
}

// AtIndex returns the chunk stored at the given index in the
// queue
func AtIndex(index int64) (*flow.Chunk, error) {
	return nil, fmt.Errorf("TBI")
}
