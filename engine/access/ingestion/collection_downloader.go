package ingestion

import (
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

type CollectionDownloader struct {
	component.Component

	ready chan struct{}
	done  chan struct{}

	logger                  zerolog.Logger
	state                   protocol.State
	requester               module.Requester
	blockStorage            storage.Blocks
	collectionStorage       storage.Collections
	transactionStorage      storage.Transactions
	executionReceiptStorage storage.ExecutionReceipts
	executionResultStorage  storage.ExecutionResults

	collectionExecutedMetric module.CollectionExecutedMetric

	lastFullBlockHeight *counters.PersistentStrictMonotonicCounter
}

var _ component.Component = (*CollectionDownloader)(nil)

func NewCollectionDownloader(
	logger zerolog.Logger,
	requester module.Requester,
	state protocol.State,
	blockStorage storage.Blocks,
	collectionStorage storage.Collections,
	transactionStorage storage.Transactions,
	receiptsStorage storage.ExecutionReceipts,
	resultsStorage storage.ExecutionResults,
	collectionExecutedMetric module.CollectionExecutedMetric,
	lastFullBlockHeight *counters.PersistentStrictMonotonicCounter,
) *CollectionDownloader {
	return &CollectionDownloader{
		ready:                    make(chan struct{}, 1),
		done:                     make(chan struct{}, 1),
		logger:                   logger,
		state:                    state,
		requester:                requester,
		blockStorage:             blockStorage,
		collectionStorage:        collectionStorage,
		transactionStorage:       transactionStorage,
		executionReceiptStorage:  receiptsStorage,
		executionResultStorage:   resultsStorage,
		lastFullBlockHeight:      lastFullBlockHeight,
		collectionExecutedMetric: collectionExecutedMetric,
	}
}

func (r *CollectionDownloader) Start(ctx irrecoverable.SignalerContext) {
	defer close(r.done)
	r.ready <- struct{}{}

	// on start-up, AN wants to download all missing collections to serve it to end users
	err := r.downloadMissingCollectionsBlocking(ctx)
	if err != nil {
		r.logger.Error().Err(err).Msg("error downloading missing collections")
	}

	downloadCollectionsInterval := time.NewTicker(time.Minute)
	for {
		select {
		case <-downloadCollectionsInterval.C:
			err := r.downloadMissingCollections()
			if err != nil {
				r.logger.Error().Err(err).Msg("error downloading missing collections")
				return
			}

		case <-ctx.Done():
			return
		}
	}
}

func (r *CollectionDownloader) Ready() <-chan struct{} {
	return r.ready
}

func (r *CollectionDownloader) Done() <-chan struct{} {
	return r.done
}

// downloadMissingCollections triggers missing collections downloading process.
func (r *CollectionDownloader) downloadMissingCollections() error {
	lastFullBlockHeight := r.lastFullBlockHeight.Value() + 1

	finalizedBlock, err := r.state.Final().Head()
	if err != nil {
		return fmt.Errorf("failed to get finalized block: %w", err)
	}
	finalizedBlockHeight := finalizedBlock.Height

	collections, incompleteBlocksCount, err := r.findMissingCollections(r.lastFullBlockHeight.Value())
	if err != nil {
		return err
	}

	blocksThresholdReached := incompleteBlocksCount > defaultMissingCollsForBlkThreshold
	ageThresholdReached := finalizedBlockHeight-lastFullBlockHeight > defaultMissingCollsForAgeThreshold
	shouldDownload := blocksThresholdReached || ageThresholdReached

	if shouldDownload {
		r.downloadCollections(collections, false)
	}

	return nil
}

// downloadMissingCollectionsBlocking triggers missing collections downloading process and blocks until it is finished.
func (r *CollectionDownloader) downloadMissingCollectionsBlocking(ctx irrecoverable.SignalerContext) error {
	collections, _, err := r.findMissingCollections(r.lastFullBlockHeight.Value())
	if err != nil {
		return err
	}

	r.downloadCollections(collections, true)

	// TODO: come up with better naming
	collectionsToBeDownloaded := make(map[flow.Identifier]struct{})
	for _, collection := range collections {
		collectionsToBeDownloaded[collection.CollectionID] = struct{}{}
	}

	// wait for all collections to be downloaded
	collectionStoragePollInterval := time.NewTicker(10 * time.Second)
	for {
		select {
		case <-collectionStoragePollInterval.C:
			for collectionID := range collectionsToBeDownloaded {
				downloaded, err := r.collectionInStorage(collectionID)
				if err != nil {
					return err
				}

				if downloaded {
					delete(collectionsToBeDownloaded, collectionID)
				}
			}

		case <-ctx.Done():
			close(r.done)
			return nil
		}
	}
}

func (r *CollectionDownloader) findMissingCollections(lastFullBlockHeight uint64) ([]*flow.CollectionGuarantee, int, error) {
	// first block to look up collections at
	firstBlockHeight := lastFullBlockHeight + 1

	lastFinalizedBlock, err := r.state.Final().Head()
	if err != nil {
		return nil, 0, err
	}
	// last block to look up collections at
	lastBlockHeight := lastFinalizedBlock.Height

	var missingCollections []*flow.CollectionGuarantee
	var incompleteBlocksCount int

	for currBlockHeight := firstBlockHeight; currBlockHeight <= lastBlockHeight; currBlockHeight++ {
		collections, err := r.findMissingCollectionsAtHeight(currBlockHeight)
		if err != nil {
			return nil, 0, err
		}

		missingCollections = append(missingCollections, collections...)
		incompleteBlocksCount += 1
	}

	return missingCollections, incompleteBlocksCount, nil
}

// findMissingCollectionsAtHeight looks for the collections at the given block height in the collection storage
func (r *CollectionDownloader) findMissingCollectionsAtHeight(height uint64) ([]*flow.CollectionGuarantee, error) {
	block, err := r.blockStorage.ByHeight(height)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve block by height %d: %w", height, err)
	}

	var missingCollections []*flow.CollectionGuarantee
	for _, guarantee := range block.Payload.Guarantees {
		inStorage, err := r.collectionInStorage(guarantee.CollectionID)
		if err != nil {
			return nil, err
		}

		if !inStorage {
			missingCollections = append(missingCollections, guarantee)
		}
	}

	return missingCollections, nil
}

// collectionInStorage looks up the collection from the collection DB
func (r *CollectionDownloader) collectionInStorage(collectionID flow.Identifier) (bool, error) {
	_, err := r.collectionStorage.LightByID(collectionID)
	if err == nil {
		return true, nil
	}

	if errors.Is(err, storage.ErrNotFound) {
		return false, nil
	}

	return false, fmt.Errorf("failed to retrieve collection %s: %w", collectionID.String(), err)
}

// downloadCollections registers download request in the requester engine to fetch collections from the network
func (r *CollectionDownloader) downloadCollections(missingCollections []*flow.CollectionGuarantee, immediately bool) {
	for _, collection := range missingCollections {
		guarantors, err := protocol.FindGuarantors(r.state, collection)
		if err != nil {
			// failed to find guarantors for guarantees contained in a finalized block is fatal error
			r.logger.Fatal().Err(err).Msgf("could not find guarantors for guarantee %v", collection.ID())
		}
		r.requester.EntityByID(collection.ID(), filter.HasNodeID[flow.Identity](guarantors...))
	}

	if immediately {
		r.requester.Force()
	}
}

// OnCollectionDownloaded indexes and persist collection after it has been downloaded. This function should be
// registered in a place where we know that a collection has been fetched successfully. At the moment, such place is a
// requester engine. The function's signature is dictated by the callback type the requester engine expects.
func (r *CollectionDownloader) OnCollectionDownloaded(_ flow.Identifier, entity flow.Entity) {
	collection, ok := entity.(*flow.Collection)
	if !ok {
		r.logger.Error().Msgf("invalid entity type (%T)", entity)
		return
	}

	err := indexer.IndexCollection(collection, r.collectionStorage, r.transactionStorage, r.logger, r.collectionExecutedMetric)
	if err != nil {
		r.logger.Error().Err(err).Msg("could not index collection after it has been downloaded")
		return
	}
}

func (r *CollectionDownloader) OnFinalizedBlock(block *flow.Block) {

}

// updateLastFullBlockReceivedIndex finds the next highest height where all previous collections
// have been indexed, and updates the LastFullBlockReceived index to that height
func (r *CollectionDownloader) updateLastFullBlockReceivedIndex() error {
	lastFullBlockHeight := r.lastFullBlockHeight.Value()

	finalizedBlock, err := r.state.Final().Head()
	if err != nil {
		return fmt.Errorf("failed to get finalized block: %w", err)
	}
	finalizedHeight := finalizedBlock.Height

	// track the latest contiguous full height
	newLastFullBlockHeight, err := r.lowestBlockHeightWithMissingCollections(lastFullBlockHeight, finalizedHeight)
	if err != nil {
		return fmt.Errorf("failed to find last full block received height: %w", err)
	}

	// if more contiguous blocks are now complete, update db
	if newLastFullBlockHeight > lastFullBlockHeight {
		err := r.lastFullBlockHeight.Set(newLastFullBlockHeight)
		if err != nil {
			return fmt.Errorf("failed to update last full block height: %w", err)
		}

		r.collectionExecutedMetric.UpdateLastFullBlockHeight(newLastFullBlockHeight)

		r.logger.Debug().
			Uint64("last_full_block_height", newLastFullBlockHeight).
			Msg("updated LastFullBlockReceived index")
	}

	return nil
}

func (r *CollectionDownloader) lowestBlockHeightWithMissingCollections(lastKnownFullBlockHeight uint64, finalizedBlockHeight uint64) (uint64, error) {
	newLastFullBlockHeight := lastKnownFullBlockHeight

	for currBlockHeight := lastKnownFullBlockHeight + 1; currBlockHeight <= finalizedBlockHeight; currBlockHeight++ {
		missingCollections, err := r.findMissingCollectionsAtHeight(currBlockHeight)
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
