package store

import (
	"errors"
	"fmt"
	"sync"

	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

// ChunksQueue stores a queue of chunk locators that assigned to me to verify.
// Job consumers can read the locators as job from the queue by index.
// Chunk locators stored in this queue are unique.
type ChunksQueue struct {
	db                storage.DB
	chunkLocatorCache *Cache[uint64, *chunks.Locator] // cache for chunk locators, indexed by job index
	storing           *sync.Mutex
}

const JobQueueChunksQueue = "JobQueueChunksQueue"
const DefaultChunkQueuesCacheSize = uint(1000)

func newChunkLocatorCache(collector module.CacheMetrics) *Cache[uint64, *chunks.Locator] {
	store := func(rw storage.ReaderBatchWriter, index uint64, locator *chunks.Locator) error {
		// make sure the chunk locator is unique
		err := operation.InsertChunkLocator(rw.Writer(), locator)
		if err != nil {
			return fmt.Errorf("failed to insert chunk locator: %w", err)
		}

		err = operation.InsertJobAtIndex(rw.Writer(), JobQueueChunksQueue, index, locator.ID())
		if err != nil {
			return fmt.Errorf("failed to set job index for chunk locator queue at index %v: %w", index, err)
		}

		return nil
	}

	retrieve := func(r storage.Reader, index uint64) (*chunks.Locator, error) {
		var locatorID flow.Identifier
		err := operation.RetrieveJobAtIndex(r, JobQueueChunksQueue, index, &locatorID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve chunk locator in queue: %w", err)
		}

		var locator chunks.Locator
		err = operation.RetrieveChunkLocator(r, locatorID, &locator)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve locator for chunk id %v: %w", locatorID, err)
		}

		return &locator, nil
	}
	return newCache(collector, metrics.ResourceChunkLocators,
		withLimit[uint64, *chunks.Locator](DefaultChunkQueuesCacheSize),
		withStore(store),
		withRetrieve(retrieve))
}

// NewChunkQueue will initialize the underlying badger database of chunk locator queue.
func NewChunkQueue(collector module.CacheMetrics, db storage.DB) *ChunksQueue {
	return &ChunksQueue{
		db:                db,
		chunkLocatorCache: newChunkLocatorCache(collector),
		storing:           &sync.Mutex{},
	}
}

// Init initializes chunk queue's latest index with the given default index.
// It returns (false, nil) if the chunk queue is already initialized.
// It returns (true, nil) if the chunk queue is successfully initialized.
func (q *ChunksQueue) Init(defaultIndex uint64) (bool, error) {
	q.storing.Lock()
	defer q.storing.Unlock()

	_, err := q.LatestIndex()
	if err == nil {
		// the chunk queue is already initialized
		return false, nil
	}

	if !errors.Is(err, storage.ErrNotFound) {
		return false, fmt.Errorf("could not get latest index: %w", err)
	}

	// the latest index does not exist,
	// if the latest index is not found, initialize it with the default index
	// in this case, double check that no chunk locator exist at the default index
	_, err = q.AtIndex(defaultIndex)
	if err == nil {
		return false, fmt.Errorf("chunk locator already exists at default index %v", defaultIndex)
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return false, fmt.Errorf("could not check chunk locator at default index %v: %w", defaultIndex, err)
	}

	// set the default index as the latest index
	err = q.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return operation.SetJobLatestIndex(rw.Writer(), JobQueueChunksQueue, defaultIndex)
	})

	if err != nil {
		return false, fmt.Errorf("could not init chunk locator queue with default index %v: %w", defaultIndex, err)
	}

	return true, nil
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
	latest, err := q.LatestIndex()
	if err != nil {
		return false, err
	}

	reader, err := q.db.Reader()
	if err != nil {
		return false, err
	}

	// make sure the chunk locator is unique
	exists, err := operation.ExistChunkLocator(reader, locator.ID())
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
		// store and cache the chunk locator
		err := q.chunkLocatorCache.PutTx(rw, next, locator)
		if err != nil {
			return fmt.Errorf("failed to store and cache chunk locator: %w", err)
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
	reader, err := q.db.Reader()
	if err != nil {
		return 0, err
	}

	var latest uint64
	err = operation.RetrieveJobLatestIndex(reader, JobQueueChunksQueue, &latest)
	if err != nil {
		return 0, fmt.Errorf("could not retrieve latest index for chunks queue: %w", err)
	}
	return latest, nil
}

// AtIndex returns the chunk locator stored at the given index in the queue.
func (q *ChunksQueue) AtIndex(index uint64) (*chunks.Locator, error) {
	reader, err := q.db.Reader()
	if err != nil {
		return nil, err
	}
	return q.chunkLocatorCache.Get(reader, index)
}
