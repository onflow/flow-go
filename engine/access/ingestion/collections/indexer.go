package collections

import (
	"errors"
	"fmt"
	"time"

	"github.com/jordanschalm/lockctx"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

const (
	// lastFullBlockRefreshInterval is the interval at which the last full block height is updated.
	lastFullBlockRefreshInterval = 1 * time.Second

	// defaultQueueCapacity is the default capacity of the pending collections queue.
	defaultQueueCapacity = 10_000
)

type CollectionIndexer interface {
	// OnCollectionReceived notifies the collection indexer that a new collection is available to be indexed.
	// Calling this method multiple times with the same collection is a no-op.
	// This method is non-blocking.
	OnCollectionReceived(collection *flow.Collection)

	// IndexCollections indexes a set of collections, skipping any collections which already exist in storage.
	// Calling this method multiple times with the same collections is a no-op.
	//
	// No error returns are expected during normal operation.
	IndexCollections(collections []*flow.Collection) error

	// MissingCollectionsAtHeight returns all collections that are not present in storage for a specific
	// finalized block height.
	//
	// Expected error returns during normal operation:
	//   - [storage.ErrNotFound]: if provided block height is not finalized or below this node's root block
	MissingCollectionsAtHeight(height uint64) ([]*flow.CollectionGuarantee, error)
}

// Indexer stores and indexes collections received from the network. It is designed to be the central
// point for accumulating collections from various subsystems that my receive them from the network.
// For example, collections may be received from execution data sync, the collection syncer, or the
// execution state indexer. Depending on the node's configuration, one or more of these subsystems
// will feed the indexer with collections.
//
// The indexer also maintains the last full block height state, which is the highest block height
// for which all collections are stored and indexed.
type Indexer struct {
	log     zerolog.Logger
	metrics module.CollectionExecutedMetric

	db                  storage.DB
	lockManager         lockctx.Manager
	state               protocol.State
	blocks              storage.Blocks
	collections         storage.Collections
	lastFullBlockHeight *counters.PersistentStrictMonotonicCounter

	pendingCollectionsNotifier engine.Notifier
	pendingCollectionsQueue    *fifoqueue.FifoQueue
}

// NewIndexer creates a new Indexer.
//
// No error returns are expected during normal operation.
func NewIndexer(
	log zerolog.Logger,
	db storage.DB,
	metrics module.CollectionExecutedMetric,
	state protocol.State,
	blocks storage.Blocks,
	collections storage.Collections,
	lastFullBlockHeight *counters.PersistentStrictMonotonicCounter,
	lockManager lockctx.Manager,
) (*Indexer, error) {
	metrics.UpdateLastFullBlockHeight(lastFullBlockHeight.Value())

	collectionsQueue, err := fifoqueue.NewFifoQueue(defaultQueueCapacity)
	if err != nil {
		return nil, fmt.Errorf("could not create collections queue: %w", err)
	}

	return &Indexer{
		log:                        log.With().Str("component", "collection-indexer").Logger(),
		metrics:                    metrics,
		db:                         db,
		lockManager:                lockManager,
		state:                      state,
		blocks:                     blocks,
		collections:                collections,
		lastFullBlockHeight:        lastFullBlockHeight,
		pendingCollectionsNotifier: engine.NewNotifier(),
		pendingCollectionsQueue:    collectionsQueue,
	}, nil
}

// WorkerLoop is a [component.ComponentWorker] that continuously processes collections submitted to
// the indexer and maintains the last full block height state.
//
// There should only be a single instance of this method running at a time.
func (ci *Indexer) WorkerLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	updateLastFullBlockHeightTicker := time.NewTicker(lastFullBlockRefreshInterval)
	defer updateLastFullBlockHeightTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-updateLastFullBlockHeightTicker.C:
			err := ci.updateLastFullBlockHeight()
			if err != nil {
				ctx.Throw(fmt.Errorf("failed to update last full block height: %w", err))
				return
			}

		case <-ci.pendingCollectionsNotifier.Channel():
			for {
				v, ok := ci.pendingCollectionsQueue.Pop()
				if !ok {
					break // no more pending collections
				}

				collection, ok := v.(*flow.Collection)
				if !ok {
					ctx.Throw(fmt.Errorf("collection indexer received invalid object. expected *flow.Collection, got: %T", collection))
					return
				}

				if err := ci.IndexCollections([]*flow.Collection{collection}); err != nil {
					ctx.Throw(fmt.Errorf("error indexing collection: %w", err))
					return
				}
			}
		}
	}
}

// OnCollectionReceived notifies the collection indexer that a new collection is available to be indexed.
// Calling this method multiple times with the same collection is a no-op.
// This method is non-blocking.
func (ci *Indexer) OnCollectionReceived(collection *flow.Collection) {
	if !ci.pendingCollectionsQueue.Push(collection) {
		ci.log.Warn().
			Hex("collection_id", logging.ID(collection.ID())).
			Msg("dropping collection because queue is full")
		return
	}
	ci.pendingCollectionsNotifier.Notify()
}

// IndexCollections indexes a set of collections, skipping any collections which already exist in storage.
// Calling this method multiple times with the same collections is a no-op.
//
// No error returns are expected during normal operation.
func (ci *Indexer) IndexCollections(collections []*flow.Collection) error {
	// skip indexing if collection is already indexed. on the common path, collections may be received
	// via multiple subsystems (e.g. execution data sync, collection sync, execution state indexer).
	// In this case, the indexer will be notified multiple times for the same collection. Only the
	// first notification should be processed.
	//
	// It's OK that this check is not done atomically with the index operation since the collections
	// storage module is solely responsible for enforcing consistency (even if this is a stale read).
	newCollections := make([]*flow.Collection, 0)
	for _, collection := range collections {
		exists, err := ci.isCollectionInStorage(collection.ID())
		if err != nil {
			return fmt.Errorf("failed to check if collection is in storage: %w", err)
		}
		if !exists {
			newCollections = append(newCollections, collection)
		}
	}

	if len(newCollections) == 0 {
		return nil
	}

	return storage.WithLock(ci.lockManager, storage.LockInsertCollection, func(lctx lockctx.Context) error {
		return ci.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			for _, collection := range newCollections {
				// store the collection, including constituent transactions, and index transactionID -> collectionID
				light, err := ci.collections.BatchStoreAndIndexByTransaction(lctx, collection, rw)
				if err != nil {
					return fmt.Errorf("failed to store collection: %w", err)
				}

				ci.metrics.CollectionFinalized(light)
				ci.metrics.CollectionExecuted(light)
			}
			return nil
		})
	})
}

// updateLastFullBlockHeight updates the LastFullBlockHeight index (if it has changed).
// The LastFullBlockHeight index tracks the height of the highest block B, such that all collections
// in B and in all B's ancestors have been indexed by this node.
//
// No error returns are expected during normal operation.
func (ci *Indexer) updateLastFullBlockHeight() error {
	lastFullBlockHeight := ci.lastFullBlockHeight.Value()
	lastFinalizedBlock, err := ci.state.Final().Head()
	if err != nil {
		return fmt.Errorf("failed to get finalized block: %w", err)
	}

	newLastFullBlockHeight := lastFullBlockHeight
	for height := lastFullBlockHeight + 1; height <= lastFinalizedBlock.Height; height++ {
		missingCollections, err := ci.MissingCollectionsAtHeight(height)
		if err != nil {
			// no errors are expected since all blocks are finalized and must be present in storage
			return fmt.Errorf("failed to retrieve missing collections for block height %d: %w", height, err)
		}

		// stop when we find the first block with missing collections
		if len(missingCollections) > 0 {
			break
		}

		newLastFullBlockHeight = height
	}

	if newLastFullBlockHeight > lastFullBlockHeight {
		err = ci.lastFullBlockHeight.Set(newLastFullBlockHeight)
		if err != nil {
			return fmt.Errorf("failed to update last full block height: %w", err)
		}

		ci.metrics.UpdateLastFullBlockHeight(newLastFullBlockHeight)

		ci.log.Debug().
			Uint64("old", lastFullBlockHeight).
			Uint64("new", newLastFullBlockHeight).
			Msg("updated last full block height counter")
	}

	return nil
}

// MissingCollectionsAtHeight returns all collections that are not present in storage for a specific
// finalized block height.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound]: if provided block height is not finalized
func (ci *Indexer) MissingCollectionsAtHeight(height uint64) ([]*flow.CollectionGuarantee, error) {
	block, err := ci.blocks.ByHeight(height)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve block by height %d: %w", height, err)
	}

	var missingCollections []*flow.CollectionGuarantee
	for _, guarantee := range block.Payload.Guarantees {
		inStorage, err := ci.isCollectionInStorage(guarantee.CollectionID)
		if err != nil {
			return nil, err
		}

		if !inStorage {
			missingCollections = append(missingCollections, guarantee)
		}
	}

	return missingCollections, nil
}

// isCollectionInStorage checks whether the given collection is present in local storage.
//
// No error returns are expected during normal operation.
func (ci *Indexer) isCollectionInStorage(collectionID flow.Identifier) (bool, error) {
	_, err := ci.collections.LightByID(collectionID)
	if err == nil {
		return true, nil
	}

	if errors.Is(err, storage.ErrNotFound) {
		return false, nil
	}

	return false, fmt.Errorf("failed to retrieve collection %s: %w", collectionID.String(), err)
}
