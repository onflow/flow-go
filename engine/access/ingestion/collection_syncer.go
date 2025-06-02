package ingestion

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

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

// TODO: why don't use constants by default?
var (
	defaultCollectionCatchupTimeout               = collectionCatchupTimeout
	defaultCollectionCatchupDBPollInterval        = collectionCatchupDBPollInterval
	defaultFullBlockRefreshInterval               = fullBlockRefreshInterval
	defaultMissingCollsRequestInterval            = missingCollsRequestInterval
	defaultMissingCollsForBlockThreshold          = missingCollsForBlockThreshold
	defaultMissingCollsForAgeThreshold     uint64 = missingCollsForAgeThreshold
)

type CollectionSyncer struct {
	logger                   zerolog.Logger
	collectionExecutedMetric module.CollectionExecutedMetric

	state     protocol.State
	requester module.Requester

	blocks       storage.Blocks
	collections  storage.Collections
	transactions storage.Transactions

	//TODO: is full block a common term in this bounded context?
	// maybe rename to lastSyncedBlockHeight? in this context it looks reasonable
	lastFullBlockHeight *counters.PersistentStrictMonotonicCounter
}

func NewCollectionSyncer(
	logger zerolog.Logger,
	collectionExecutedMetric module.CollectionExecutedMetric,
	requester module.Requester,
	state protocol.State,
	blocks storage.Blocks,
	collections storage.Collections,
	transactions storage.Transactions,
	lastFullBlockHeight *counters.PersistentStrictMonotonicCounter,
) *CollectionSyncer {
	collectionExecutedMetric.UpdateLastFullBlockHeight(lastFullBlockHeight.Value())

	return &CollectionSyncer{
		logger:                   logger,
		state:                    state,
		requester:                requester,
		blocks:                   blocks,
		collections:              collections,
		transactions:             transactions,
		lastFullBlockHeight:      lastFullBlockHeight,
		collectionExecutedMetric: collectionExecutedMetric,
	}
}

func (s *CollectionSyncer) RequestCollections(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	requestCtx, cancel := context.WithTimeout(ctx, defaultCollectionCatchupTimeout)
	defer cancel()

	// on start-up, AN wants to download all missing collections to serve it to end users
	err := s.downloadMissingCollections(requestCtx)
	if err != nil {
		s.logger.Error().Err(err).Msg("error downloading missing collections")
	}
	ready()

	requestCollectionsInterval := time.NewTicker(defaultMissingCollsRequestInterval)
	defer requestCollectionsInterval.Stop()

	updateLastFullBlockHeightInternal := time.NewTicker(defaultFullBlockRefreshInterval)
	defer updateLastFullBlockHeightInternal.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-requestCollectionsInterval.C:
			err := s.requestMissingCollections()
			if err != nil {
				s.logger.Error().Err(err).Msg("error requesting missing collections")
				return
			}

		case <-updateLastFullBlockHeightInternal.C:
			err := s.updateLastFullBlockHeight()
			if err != nil {
				s.logger.Error().Err(err).Msg("error updating last full block height")
			}
		}
	}
}

// requestMissingCollections triggers missing collections downloading process.
func (s *CollectionSyncer) requestMissingCollections() error {
	lastFullBlockHeight := s.lastFullBlockHeight.Value() + 1
	lastFinalizedBlock, err := s.state.Final().Head()
	if err != nil {
		return fmt.Errorf("failed to get finalized block: %w", err)
	}

	collections, incompleteBlocksCount, err := s.findMissingCollections(s.lastFullBlockHeight.Value())
	if err != nil {
		return err
	}

	blocksThresholdReached := incompleteBlocksCount > defaultMissingCollsForBlockThreshold
	ageThresholdReached := lastFinalizedBlock.Height-lastFullBlockHeight > defaultMissingCollsForAgeThreshold
	shouldRequest := blocksThresholdReached || ageThresholdReached

	if shouldRequest {
		s.requestCollections(collections, false)
	}

	return nil
}

// downloadMissingCollections triggers missing collections downloading process and blocks until it is finished.
func (s *CollectionSyncer) downloadMissingCollections(ctx context.Context) error {
	collections, _, err := s.findMissingCollections(s.lastFullBlockHeight.Value())
	if err != nil {
		return err
	}

	s.requestCollections(collections, true)

	collectionsToBeDownloaded := make(map[flow.Identifier]struct{})
	for _, collection := range collections {
		collectionsToBeDownloaded[collection.CollectionID] = struct{}{}
	}

	// wait for all collections to be downloaded
	collectionStoragePollInterval := time.NewTicker(defaultCollectionCatchupDBPollInterval)
	for {
		select {
		case <-ctx.Done():
			return nil

		case <-collectionStoragePollInterval.C:
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
}

func (s *CollectionSyncer) findMissingCollections(lastFullBlockHeight uint64) ([]*flow.CollectionGuarantee, int, error) {
	// first block to look up collections at
	firstBlockHeight := lastFullBlockHeight + 1

	lastFinalizedBlock, err := s.state.Final().Head()
	if err != nil {
		return nil, 0, err
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

		missingCollections = append(missingCollections, collections...)
		incompleteBlocksCount += 1
	}

	return missingCollections, incompleteBlocksCount, nil
}

// findMissingCollectionsAtHeight looks for the collections at the given block height in the collection storage
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

// isCollectionInStorage looks up the collection from the collection DB
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

func (s *CollectionSyncer) RequestCollectionsForBlock(height uint64, missingCollections []*flow.CollectionGuarantee, immediately bool) {
	if height <= s.lastFullBlockHeight.Value() {
		return
	}

	s.requestCollections(missingCollections, immediately)
}

// requestCollections registers download request in the requester engine to fetch collections from the network
func (s *CollectionSyncer) requestCollections(missingCollections []*flow.CollectionGuarantee, immediately bool) {
	for _, collection := range missingCollections {
		guarantors, err := protocol.FindGuarantors(s.state, collection)
		if err != nil {
			// failed to find guarantors for guarantees contained in a finalized block is fatal error
			s.logger.Fatal().Err(err).Msgf("could not find guarantors for guarantee %v", collection.ID())
		}
		s.requester.EntityByID(collection.ID(), filter.HasNodeID[flow.Identity](guarantors...))
	}

	if immediately {
		s.requester.Force()
	}
}

// updateLastFullBlockHeight finds and updates the next highest height where
// all previous collections have been indexed
func (s *CollectionSyncer) updateLastFullBlockHeight() error {
	lastFullBlockHeight := s.lastFullBlockHeight.Value()
	lastFinalizedBlock, err := s.state.Final().Head()
	if err != nil {
		return fmt.Errorf("failed to get finalized block: %w", err)
	}

	// track the latest contiguous full height
	newLastFullBlockHeight, err := s.findLowestBlockHeightWithMissingCollections(lastFullBlockHeight, lastFinalizedBlock.Height)
	if err != nil {
		return fmt.Errorf("failed to find last full block received height: %w", err)
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

func (s *CollectionSyncer) findLowestBlockHeightWithMissingCollections(lastKnownFullBlockHeight uint64, finalizedBlockHeight uint64) (uint64, error) {
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

// OnCollectionDownloaded indexes and persist collection after it has been downloaded. This function should be
// registered in a place where we know that a collection has been fetched successfully. At the moment, such place is a
// requester engine. The function's signature is dictated by the callback type the requester engine expects.
func (s *CollectionSyncer) OnCollectionDownloaded(_ flow.Identifier, entity flow.Entity) {
	collection, ok := entity.(*flow.Collection)
	if !ok {
		s.logger.Error().Msgf("invalid entity type (%T)", entity)
		return
	}

	// TODO: we could wrap the collection storage with a thin abstraction that puts a collection ID to a channel when it
	// has been downloaded. Then we could listen to this channel instead of polling storage periodically as we do atm.
	//
	// However, as I understand, we're not interested in the collection retrieval itself, we're interested of the fact
	// that it has been saved in the storage, so we can serve it to the end users. Correct? (ask Peter)
	//
	// But I'm curious whether we can rely on the fact that indexer returns an error if sth goes wrong.
	// If indexer didn't return error on indexing, does it mean the item has been successfully saved into the storage?
	// I guess no because it doesn't align with the fact that we use batch writes...
	// Can indexer signal when the batch write finished successfully though?
	// I guess I need more knowledge of how indexer works.. (ask Peter)
	err := indexer.IndexCollection(collection, s.collections, s.transactions, s.logger, s.collectionExecutedMetric)
	if err != nil {
		s.logger.Error().Err(err).Msg("could not index collection after it has been downloaded")
		return
	}
}
