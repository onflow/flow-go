package ingestion2

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

const (
	// time to wait for the all the missing collections to be received at node startup
	collectionCatchupTimeout = 30 * time.Second

	// time to poll the storage to check if missing collections have been received
	collectionCatchupDBPollInterval = 10 * time.Millisecond

	// time to request missing collections from the network
	missingCollsRequestInterval = 1 * time.Minute

	// a threshold of number of blocks with missing collections beyond which collections should be re-requested
	// this is to prevent spamming the collection nodes with request
	missingCollsForBlockThreshold = 100

	// a threshold of block height beyond which collections should be re-requested (regardless of the number of blocks for which collection are missing)
	// this is to ensure that if a collection is missing for a long time (in terms of block height) it is eventually re-requested
	missingCollsForAgeThreshold = 100

	// time to update the FullBlockHeight index
	fullBlockRefreshInterval = 1 * time.Second
)

var (
	// we change these values in tests. that's why we want to have their non-const shape
	defaultMissingCollsForBlockThreshold        = missingCollsForBlockThreshold
	defaultMissingCollsForAgeThreshold   uint64 = missingCollsForAgeThreshold
)

// The CollectionSyncer type provides mechanisms for syncing and indexing data
// from the Flow blockchain into local storage. Specifically, it handles
// the retrieval and processing of collections and transactions that may
// have been missed due to network delays, restarts, or gaps in finalization.
//
// It is responsible for ensuring the local node has
// all collections associated with finalized blocks starting from the
// last fully synced height. It works by periodically scanning the finalized
// block range, identifying missing collections, and triggering requests
// to fetch them from the network. Once collections are retrieved, it
// ensures they are persisted in the local collection and transaction stores.
//
// The syncer maintains a persistent, strictly monotonic counter
// (`lastFullBlockHeight`) to track the highest finalized block for which
// all collections have been fully indexed. It uses this information to
// avoid redundant processing and to measure catch-up progress.
//
// It is meant to operate in a background goroutine as part of the
// node's ingestion pipeline.
type CollectionSyncer struct {
	logger                   zerolog.Logger
	collectionExecutedMetric module.CollectionExecutedMetric

	state     protocol.State
	requester module.Requester

	// collections to be indexed
	pendingCollections        *engine.FifoMessageStore
	pendingCollectionsHandler *engine.MessageHandler

	blocks      storage.Blocks
	collections storage.Collections

	lastFullBlockHeight *counters.PersistentStrictMonotonicCounter
	lockManager         storage.LockManager
}

// NewCollectionSyncer creates a new CollectionSyncer responsible for requesting,
// tracking, and indexing missing collections.
func NewCollectionSyncer(
	logger zerolog.Logger,
	collectionExecutedMetric module.CollectionExecutedMetric,
	requester module.Requester,
	state protocol.State,
	blocks storage.Blocks,
	collections storage.Collections,
	lastFullBlockHeight *counters.PersistentStrictMonotonicCounter,
	lockManager storage.LockManager,
) (*CollectionSyncer, error) {
	collectionExecutedMetric.UpdateLastFullBlockHeight(lastFullBlockHeight.Value())

	collectionsQueue, err := fifoqueue.NewFifoQueue(defaultQueueCapacity)
	if err != nil {
		return nil, fmt.Errorf("could not create collections queue: %w", err)
	}

	pendingCollections := &engine.FifoMessageStore{FifoQueue: collectionsQueue}
	pendingCollectionsHandler := engine.NewMessageHandler(
		logger,
		engine.NewNotifier(),
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				_, ok := msg.Payload.(*flow.Collection)
				return ok
			},
			Store: pendingCollections,
		},
	)

	return &CollectionSyncer{
		logger:                    logger,
		state:                     state,
		requester:                 requester,
		pendingCollectionsHandler: pendingCollectionsHandler,
		pendingCollections:        pendingCollections,
		blocks:                    blocks,
		collections:               collections,
		lastFullBlockHeight:       lastFullBlockHeight,
		collectionExecutedMetric:  collectionExecutedMetric,
		lockManager:               lockManager,
	}, nil
}

// StartWorkerLoop continuously monitors and triggers collection sync operations.
// It handles on startup collection catchup, periodic missing collection requests, and full block height updates.
func (s *CollectionSyncer) StartWorkerLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	requestCtx, cancel := context.WithTimeout(ctx, collectionCatchupTimeout)
	defer cancel()

	// on start-up, AN wants to download all missing collections to serve it to end users
	err := s.requestMissingCollectionsBlocking(requestCtx)
	if err != nil {
		s.logger.Error().Err(err).Msg("error downloading missing collections")
	}
	ready()

	requestCollectionsTicker := time.NewTicker(missingCollsRequestInterval)
	defer requestCollectionsTicker.Stop()

	// Collections are requested concurrently in this design.
	// To maintain accurate progress tracking and avoid redundant requests,
	// we periodically update the `lastFullBlockHeight` to reflect the latest
	// finalized block with all collections successfully indexed.
	updateLastFullBlockHeightTicker := time.NewTicker(fullBlockRefreshInterval)
	defer updateLastFullBlockHeightTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-requestCollectionsTicker.C:
			err := s.requestMissingCollections()
			if err != nil {
				ctx.Throw(err)
			}

		case <-updateLastFullBlockHeightTicker.C:
			err := s.updateLastFullBlockHeight()
			if err != nil {
				ctx.Throw(err)
			}

		case <-s.pendingCollectionsHandler.GetNotifier():
			msg, ok := s.pendingCollections.Get()
			if !ok {
				ctx.Throw(fmt.Errorf("could not get pending collection"))
			}

			collection, ok := msg.Payload.(*flow.Collection)
			if !ok {
				ctx.Throw(fmt.Errorf("could not cast pending collection to *flow.Collection. got: %T", msg.Payload))
				return
			}

			// Create a lock context for indexing
			lctx := s.lockManager.NewContext()
			err := lctx.AcquireLock(storage.LockInsertCollection)
			if err != nil {
				ctx.Throw(fmt.Errorf("could not acquire lock for collection indexing: %w", err))
				return
			}
			defer lctx.Release()

			err = indexer.IndexCollection(lctx, collection, s.collections, s.logger, s.collectionExecutedMetric)
			if err != nil {
				ctx.Throw(fmt.Errorf("error indexing collection: %w", err))
				return
			}
		}
	}
}

// requestMissingCollections checks if missing collections should be requested based on configured
// block or age thresholds and triggers requests if needed.
//
// No errors are expected during normal operations.
func (s *CollectionSyncer) requestMissingCollections() error {
	lastFullBlockHeight := s.lastFullBlockHeight.Value()
	lastFinalizedBlock, err := s.state.Final().Head()
	if err != nil {
		return fmt.Errorf("failed to get finalized block: %w", err)
	}

	collections, incompleteBlocksCount, err := s.findMissingCollections(lastFullBlockHeight)
	if err != nil {
		return err
	}

	blocksThresholdReached := incompleteBlocksCount >= defaultMissingCollsForBlockThreshold
	ageThresholdReached := lastFinalizedBlock.Height-lastFullBlockHeight > defaultMissingCollsForAgeThreshold
	shouldRequest := blocksThresholdReached || ageThresholdReached

	if shouldRequest {
		// warn log since generally this should not happen
		s.logger.Warn().
			Uint64("finalized_height", lastFinalizedBlock.Height).
			Uint64("last_full_blk_height", lastFullBlockHeight).
			Int("missing_collection_blk_count", incompleteBlocksCount).
			Int("missing_collection_count", len(collections)).
			Msg("re-requesting missing collections")

		s.requestCollections(collections, false)
	}

	return nil
}

// requestMissingCollectionsBlocking requests and waits for all missing collections to be downloaded,
// blocking until either completion or context timeout.
//
// No errors are expected during normal operations.
func (s *CollectionSyncer) requestMissingCollectionsBlocking(ctx context.Context) error {
	missingCollections, _, err := s.findMissingCollections(s.lastFullBlockHeight.Value())
	if err != nil {
		return err
	}
	if len(missingCollections) == 0 {
		s.logger.Info().Msg("skipping requesting missing collections. no missing collections found")
		return nil
	}

	s.requestCollections(missingCollections, true)

	collectionsToBeDownloaded := make(map[flow.Identifier]struct{})
	for _, collection := range missingCollections {
		collectionsToBeDownloaded[collection.CollectionID] = struct{}{}
	}

	collectionStoragePollTicker := time.NewTicker(collectionCatchupDBPollInterval)
	defer collectionStoragePollTicker.Stop()

	// we want to wait for all collections to be downloaded so we poll local storage periodically to make sure each
	// collection was successfully saved in the storage.
	for len(collectionsToBeDownloaded) > 0 {
		select {
		case <-ctx.Done():
			return fmt.Errorf("failed to complete collection retrieval: %w", ctx.Err())

		case <-collectionStoragePollTicker.C:
			s.logger.Info().
				Int("total_missing_collections", len(collectionsToBeDownloaded)).
				Msg("retrieving missing collections...")

			for collectionID := range collectionsToBeDownloaded {
				downloaded, err := s.isCollectionInStorage(collectionID)
				if err != nil {
					return err
				}

				if downloaded {
					delete(collectionsToBeDownloaded, collectionID)
				}
			}
		}
	}

	s.logger.Info().Msg("collection catchup done")
	return nil
}

// findMissingCollections scans block heights from last known full block up to the latest finalized
// block and returns all missing collection along with the count of incomplete blocks.
//
// No errors are expected during normal operations.
func (s *CollectionSyncer) findMissingCollections(lastFullBlockHeight uint64) ([]*flow.CollectionGuarantee, int, error) {
	// first block to look up collections at
	firstBlockHeight := lastFullBlockHeight + 1

	lastFinalizedBlock, err := s.state.Final().Head()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get finalized block: %w", err)
	}
	// last block to look up collections at
	lastBlockHeight := lastFinalizedBlock.Height

	var missingCollections []*flow.CollectionGuarantee
	var incompleteBlocksCount int

	for currBlockHeight := firstBlockHeight; currBlockHeight <= lastBlockHeight; currBlockHeight++ {
		collections, err := s.findMissingCollectionsAtHeight(currBlockHeight)
		if err != nil {
			return nil, 0, err
		}

		if len(collections) == 0 {
			continue
		}

		missingCollections = append(missingCollections, collections...)
		incompleteBlocksCount += 1
	}

	return missingCollections, incompleteBlocksCount, nil
}

// findMissingCollectionsAtHeight returns all missing collections for a specific block height.
//
// No errors are expected during normal operations.
func (s *CollectionSyncer) findMissingCollectionsAtHeight(height uint64) ([]*flow.CollectionGuarantee, error) {
	block, err := s.blocks.ByHeight(height)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve block by height %d: %w", height, err)
	}

	var missingCollections []*flow.CollectionGuarantee
	for _, guarantee := range block.Payload.Guarantees {
		inStorage, err := s.isCollectionInStorage(guarantee.CollectionID)
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
// No errors are expected during normal operations.
func (s *CollectionSyncer) isCollectionInStorage(collectionID flow.Identifier) (bool, error) {
	_, err := s.collections.LightByID(collectionID)
	if err == nil {
		return true, nil
	}

	if errors.Is(err, storage.ErrNotFound) {
		return false, nil
	}

	return false, fmt.Errorf("failed to retrieve collection %s: %w", collectionID.String(), err)
}

// RequestCollectionsForBlock conditionally requests missing collections for a specific block height,
// skipping requests if the block is already below the known full block height.
func (s *CollectionSyncer) RequestCollectionsForBlock(height uint64, missingCollections []*flow.CollectionGuarantee) {
	// skip requesting collections, if this block is below the last full block height.
	// this means that either we have already received these collections, or the block
	// may contain unverifiable guarantees (in case this node has just joined the network)
	if height <= s.lastFullBlockHeight.Value() {
		s.logger.Debug().
			Msg("skipping requesting collections for finalized block as its collections have been already retrieved")
		return
	}

	s.requestCollections(missingCollections, false)
}

// requestCollections registers collection download requests in the requester engine,
// optionally forcing immediate dispatch.
func (s *CollectionSyncer) requestCollections(collections []*flow.CollectionGuarantee, immediately bool) {
	for _, guarantee := range collections {
		guarantors, err := protocol.FindGuarantors(s.state, guarantee)
		if err != nil {
			// failed to find guarantors for guarantees contained in a finalized block is fatal error
			s.logger.Fatal().Err(err).Msgf("could not find guarantors for collection %v", guarantee.CollectionID)
		}
		s.requester.EntityByID(guarantee.CollectionID, filter.HasNodeID[flow.Identity](guarantors...))
	}

	if immediately {
		s.requester.Force()
	}
}

// updateLastFullBlockHeight updates the next highest block height where all previous collections have been indexed.
//
// No errors are expected during normal operations.
func (s *CollectionSyncer) updateLastFullBlockHeight() error {
	lastFullBlockHeight := s.lastFullBlockHeight.Value()
	lastFinalizedBlock, err := s.state.Final().Head()
	if err != nil {
		return fmt.Errorf("failed to get finalized block: %w", err)
	}

	// track the latest contiguous full height
	newLastFullBlockHeight, err := s.findLowestBlockHeightWithMissingCollections(lastFullBlockHeight, lastFinalizedBlock.Height)
	if err != nil {
		return fmt.Errorf("failed to find last full block height: %w", err)
	}

	// if more contiguous blocks are now complete, update db
	if newLastFullBlockHeight > lastFullBlockHeight {
		err := s.lastFullBlockHeight.Set(newLastFullBlockHeight)
		if err != nil {
			return fmt.Errorf("failed to update last full block height: %w", err)
		}

		s.collectionExecutedMetric.UpdateLastFullBlockHeight(newLastFullBlockHeight)

		s.logger.Debug().
			Uint64("last_full_block_height", newLastFullBlockHeight).
			Msg("updated last full block height counter")
	}

	return nil
}

// findLowestBlockHeightWithMissingCollections finds the next block height with missing collections,
// returning the latest contiguous height where all collections are present.
//
// No errors are expected during normal operations.
func (s *CollectionSyncer) findLowestBlockHeightWithMissingCollections(
	lastKnownFullBlockHeight uint64,
	finalizedBlockHeight uint64,
) (uint64, error) {
	newLastFullBlockHeight := lastKnownFullBlockHeight

	for currBlockHeight := lastKnownFullBlockHeight + 1; currBlockHeight <= finalizedBlockHeight; currBlockHeight++ {
		missingCollections, err := s.findMissingCollectionsAtHeight(currBlockHeight)
		if err != nil {
			return 0, err
		}

		// return when we find the first block with missing collections
		if len(missingCollections) > 0 {
			return newLastFullBlockHeight, nil
		}

		newLastFullBlockHeight = currBlockHeight
	}

	return newLastFullBlockHeight, nil
}

// OnCollectionDownloaded indexes and persists a downloaded collection.
// This function is a callback intended to be used by the requester engine.
func (s *CollectionSyncer) OnCollectionDownloaded(id flow.Identifier, entity flow.Entity) {
	err := s.pendingCollectionsHandler.Process(id, entity)
	if err != nil {
		// this is an unexpected error condition. The only expected error returned from Process
		// is for an unexpected type. since OnCollectionDownloaded is called from the requester engine,
		// which is configured to only process collections, any error returned here indicates
		// a bug or state corruption.
		s.logger.Fatal().Err(err).Msg("failed to process pending collections")
		return
	}
}
