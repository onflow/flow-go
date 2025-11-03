package collections

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
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

const (
	// DefaultCollectionCatchupTimeout is the timeout for the initial collection catchup process
	// during startup.
	DefaultCollectionCatchupTimeout = 30 * time.Second

	// DefaultCollectionCatchupDBPollInterval is the interval at which the storage is polled to check
	// if missing collections have been received during the initial collection catchup process.
	DefaultCollectionCatchupDBPollInterval = 1 * time.Second

	// DefaultMissingCollsRequestInterval is the interval at which the syncer checks missing collections
	// and re-requests them from the network if needed.
	DefaultMissingCollsRequestInterval = 1 * time.Minute

	// DefaultMissingCollsForBlockThreshold is the threshold number of blocks with missing collections
	// beyond which collections should be re-requested. This prevents spamming the collection nodes
	// with requests for recent data.
	DefaultMissingCollsForBlockThreshold = 100

	// DefaultMissingCollsForAgeThreshold is the block height threshold below which collections
	// should be re-requested, regardless of the number of blocks for which collection are missing.
	// This is to ensure that if a collection is missing for a long time (in terms of block height)
	// it is eventually re-requested.
	DefaultMissingCollsForAgeThreshold = 100
)

// The Syncer is responsible for syncing collections for finalized blocks from the network. It has
// three main responsibilities:
//  1. Handle requests for collections for finalized blocks from the ingestion engine by submitting
//     the requests to Collection nodes.
//  2. Track blocks with missing collections, and periodically re-request the missing collections
//     from the network.
//  3. Submit collections received to the Indexer for storage and indexing.
//
// The Syncer guarantees that all collections for finalized blocks will eventually be received as long
// as there are honest Collection nodes in each cluster, and the node is able to successfully communicate
// with them over the networking layer.
//
// It is responsible for ensuring the local node has all collections contained within finalized blocks
// starting from the last fully synced height. It works by periodically scanning the finalized block
// range from the last full block height up to the latest finalized block height, identifying missing
// collections, and triggering requests to fetch them from the network. Once collections are retrieved,
// it submits them to the Indexer for storage and indexing.
//
// It is meant to operate in a background goroutine as part of the node's ingestion pipeline.
type Syncer struct {
	log zerolog.Logger

	requester           module.Requester
	indexer             CollectionIndexer
	state               protocol.State
	collections         storage.Collections
	lastFullBlockHeight counters.Reader
	execDataSyncer      *ExecutionDataSyncer // may be nil

	// these are held as members to allow configuring their values during testing.
	collectionCatchupTimeout        time.Duration
	collectionCatchupDBPollInterval time.Duration
	missingCollsForBlockThreshold   int
	missingCollsForAgeThreshold     uint64
	missingCollsRequestInterval     time.Duration
}

// NewSyncer creates a new Syncer responsible for requesting, tracking, and indexing missing collections.
func NewSyncer(
	log zerolog.Logger,
	requester module.Requester,
	state protocol.State,
	collections storage.Collections,
	lastFullBlockHeight counters.Reader,
	collectionIndexer CollectionIndexer,
	execDataSyncer *ExecutionDataSyncer,
) *Syncer {
	return &Syncer{
		log:                 log.With().Str("component", "collection-syncer").Logger(),
		state:               state,
		requester:           requester,
		collections:         collections,
		lastFullBlockHeight: lastFullBlockHeight,
		indexer:             collectionIndexer,
		execDataSyncer:      execDataSyncer,

		collectionCatchupTimeout:        DefaultCollectionCatchupTimeout,
		collectionCatchupDBPollInterval: DefaultCollectionCatchupDBPollInterval,
		missingCollsForBlockThreshold:   DefaultMissingCollsForBlockThreshold,
		missingCollsForAgeThreshold:     DefaultMissingCollsForAgeThreshold,
		missingCollsRequestInterval:     DefaultMissingCollsRequestInterval,
	}
}

// WorkerLoop is a [component.ComponentWorker] that continuously monitors for missing collections, and
// requests them from the network if needed. It also performs an initial collection catchup on startup.
func (s *Syncer) WorkerLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	requestCtx, cancel := context.WithTimeout(ctx, s.collectionCatchupTimeout)
	defer cancel()

	// attempt to download all known missing collections on start-up.
	err := s.requestMissingCollectionsBlocking(requestCtx)
	if err != nil {
		if ctx.Err() != nil {
			s.log.Error().Err(err).Msg("engine shutdown while downloading missing collections")
			return
		}

		if !errors.Is(err, context.DeadlineExceeded) {
			ctx.Throw(fmt.Errorf("error downloading missing collections: %w", err))
			return
		}

		// timed out during catchup. continue with normal startup.
		// missing collections will be requested periodically.
		s.log.Error().Err(err).Msg("timed out syncing collections during startup")
	}
	ready()

	requestCollectionsTicker := time.NewTicker(s.missingCollsRequestInterval)
	defer requestCollectionsTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-requestCollectionsTicker.C:
			err := s.requestMissingCollections(ctx)
			if err != nil {
				ctx.Throw(fmt.Errorf("failed to request missing collections: %w", err))
				return
			}
		}
	}
}

// OnCollectionDownloaded notifies the collection syncer that a collection has been downloaded.
// This callback implements [requester.HandleFunc] and is intended to be used with the requester engine.
// Panics if the provided entity is not a [flow.Collection].
func (s *Syncer) OnCollectionDownloaded(_ flow.Identifier, entity flow.Entity) {
	s.indexer.OnCollectionReceived(entity.(*flow.Collection))
}

// RequestCollectionsForBlock conditionally requests missing collections for a specific block height,
// skipping requests if the block is already below the known last full block height.
//
// No error returns are expected during normal operation.
func (s *Syncer) RequestCollectionsForBlock(height uint64, missingCollections []*flow.CollectionGuarantee) error {
	// skip requesting collections, if this block is below the last full block height.
	// this means that either we have already received these collections, or the block
	// may contain unverifiable guarantees (in case this node has just joined the network)
	if height <= s.lastFullBlockHeight.Value() {
		s.log.Debug().Msg("collections for finalized block already retrieved. skipping request.")
		return nil
	}

	err := s.requestCollections(missingCollections)
	if err != nil {
		return fmt.Errorf("failed to request collections: %w", err)
	}

	// trigger immediate dispatch of any pending collection requests.
	s.requester.Force()

	return nil
}

// requestCollections registers collection download requests in the requester engine
//
// No error returns are expected during normal operation.
func (s *Syncer) requestCollections(collections []*flow.CollectionGuarantee) error {
	for _, guarantee := range collections {
		guarantors, err := protocol.FindGuarantors(s.state, guarantee)
		if err != nil {
			// failed to find guarantors for guarantees contained in a finalized block is fatal error
			return fmt.Errorf("could not find guarantors for collection %v: %w", guarantee.CollectionID, err)
		}
		s.requester.EntityByID(guarantee.CollectionID, filter.HasNodeID[flow.Identity](guarantors...))
	}
	return nil
}

// requestMissingCollections checks if missing collections should be requested based on configured
// block or age thresholds and triggers requests if needed.
//
// No error returns are expected during normal operation.
func (s *Syncer) requestMissingCollections(ctx context.Context) error {
	lastFullBlockHeight := s.lastFullBlockHeight.Value()
	lastFinalizedBlock, err := s.state.Final().Head()
	if err != nil {
		return fmt.Errorf("failed to get finalized block: %w", err)
	}

	// if the node is syncing execution data, use the already downloaded data to index any available
	// collections we are still missing.
	lastSyncedHeight := lastFullBlockHeight
	if s.execDataSyncer != nil {
		lastSyncedHeight, err = s.execDataSyncer.IndexFromStartHeight(ctx, lastFullBlockHeight)
		if err != nil {
			return fmt.Errorf("failed to index collections from execution data: %w", err)
		}
		// At this point, all collections within blocks up to and including `lastSyncedHeight` were
		// submitted for indexing. However, indexing is completed asynchronously, so `lastSyncedHeight`
		// was set to the last block for which we have execution data to avoid re-requesting already
		// submitted collections.
	}

	// request all other missing collections from Collection nodes.
	collections, incompleteBlocksCount, err := s.findMissingCollections(lastSyncedHeight, lastFinalizedBlock.Height)
	if err != nil {
		return fmt.Errorf("failed to find missing collections: %w", err)
	}

	// only send requests if we are sufficiently behind the latest finalized block to avoid spamming
	// collection nodes with requests.
	blocksThresholdReached := incompleteBlocksCount >= s.missingCollsForBlockThreshold
	ageThresholdReached := lastFinalizedBlock.Height-lastFullBlockHeight > s.missingCollsForAgeThreshold
	if len(collections) > 0 && (blocksThresholdReached || ageThresholdReached) {
		// warn log since this should generally not happen
		s.log.Warn().
			Uint64("finalized_height", lastFinalizedBlock.Height).
			Uint64("last_full_blk_height", lastFullBlockHeight).
			Int("missing_collection_blk_count", incompleteBlocksCount).
			Int("missing_collection_count", len(collections)).
			Msg("re-requesting missing collections")

		err = s.requestCollections(collections)
		if err != nil {
			return fmt.Errorf("failed to request collections: %w", err)
		}
		// since this is a re-request, do not use force. new finalized block requests will force
		// dispatch. On the happy path, this will happen at least once per second.
	}

	return nil
}

// findMissingCollections scans block heights from last known full block up to the latest finalized
// block and returns all missing collection along with the count of incomplete blocks.
//
// No error returns are expected during normal operation.
func (s *Syncer) findMissingCollections(lastFullBlockHeight, finalizedBlockHeight uint64) ([]*flow.CollectionGuarantee, int, error) {
	var missingCollections []*flow.CollectionGuarantee
	var incompleteBlocksCount int

	for height := lastFullBlockHeight + 1; height <= finalizedBlockHeight; height++ {
		collections, err := s.indexer.MissingCollectionsAtHeight(height)
		if err != nil {
			return nil, 0, err
		}

		if len(collections) > 0 {
			missingCollections = append(missingCollections, collections...)
			incompleteBlocksCount += 1
		}
	}

	return missingCollections, incompleteBlocksCount, nil
}

// requestMissingCollectionsBlocking requests and waits for all missing collections to be downloaded,
// blocking until either completion or context timeout.
//
// No error returns are expected during normal operation.
func (s *Syncer) requestMissingCollectionsBlocking(ctx context.Context) error {
	lastFullBlockHeight := s.lastFullBlockHeight.Value()
	lastFinalizedBlock, err := s.state.Final().Head()
	if err != nil {
		return fmt.Errorf("failed to get finalized block: %w", err)
	}

	progress := util.LogProgress(s.log, util.DefaultLogProgressConfig("requesting missing collections", lastFinalizedBlock.Height-lastFullBlockHeight))

	pendingCollections := make(map[flow.Identifier]struct{})
	for height := lastFullBlockHeight + 1; height <= lastFinalizedBlock.Height; height++ {
		if ctx.Err() != nil {
			return fmt.Errorf("missing collection catchup interrupted: %w", ctx.Err())
		}

		collections, err := s.indexer.MissingCollectionsAtHeight(height)
		if err != nil {
			return fmt.Errorf("failed to find missing collections at height %d: %w", height, err)
		}

		if len(collections) > 0 {
			var submitted bool
			if s.execDataSyncer != nil {
				submitted, err = s.execDataSyncer.IndexForHeight(ctx, height)
				if err != nil {
					return fmt.Errorf("failed to index collections from execution data: %w", err)
				}
			}

			// if the data wasn't available from execution data, request it from Collection nodes.
			if !submitted {
				err = s.requestCollections(collections)
				if err != nil {
					return fmt.Errorf("failed to request collections: %w", err)
				}
				for _, collection := range collections {
					pendingCollections[collection.CollectionID] = struct{}{}
				}
			}
		}

		progress(1)
	}

	if len(pendingCollections) == 0 {
		s.log.Info().Msg("no missing collections to download")
		return nil
	}

	// trigger immediate dispatch of any pending collection requests.
	s.requester.Force()

	collectionStoragePollTicker := time.NewTicker(s.collectionCatchupDBPollInterval)
	defer collectionStoragePollTicker.Stop()

	// we want to wait for all collections to be downloaded so we poll local storage periodically to
	// make sure each collection was successfully stored and indexed.
	for len(pendingCollections) > 0 {
		select {
		case <-ctx.Done():
			return fmt.Errorf("failed to complete collection retrieval: %w", ctx.Err())

		case <-collectionStoragePollTicker.C:
			s.log.Debug().
				Int("total_missing_collections", len(pendingCollections)).
				Msg("retrieving missing collections...")

			for collectionID := range pendingCollections {
				downloaded, err := s.indexer.IsCollectionInStorage(collectionID)
				if err != nil {
					return err
				}

				if downloaded {
					delete(pendingCollections, collectionID)
				}
			}
		}
	}

	s.log.Info().Msg("collection catchup done")
	return nil
}
