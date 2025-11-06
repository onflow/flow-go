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

	// DefaultMissingCollectionRequestInterval is the interval at which the syncer checks missing collections
	// and re-requests them from the network if needed.
	DefaultMissingCollectionRequestInterval = 1 * time.Minute

	// DefaultMissingCollectionRequestThreshold is the block height threshold below which collections
	// should be re-requested, regardless of the number of blocks for which collection are missing.
	// This is to ensure that if a collection is missing for a long time (in terms of block height)
	// it is eventually re-requested.
	DefaultMissingCollectionRequestThreshold = 100
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
	collectionCatchupTimeout          time.Duration
	collectionCatchupDBPollInterval   time.Duration
	missingCollectionRequestThreshold uint64
	missingCollectionRequestInterval  time.Duration
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

		collectionCatchupTimeout:          DefaultCollectionCatchupTimeout,
		collectionCatchupDBPollInterval:   DefaultCollectionCatchupDBPollInterval,
		missingCollectionRequestThreshold: DefaultMissingCollectionRequestThreshold,
		missingCollectionRequestInterval:  DefaultMissingCollectionRequestInterval,
	}
}

// WorkerLoop is a [component.ComponentWorker] that continuously monitors for missing collections, and
// requests them from the network if needed. It also performs an initial collection catchup on startup.
func (s *Syncer) WorkerLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	// Block marking the component ready until either the first run of the missing collections catchup
	// completes, or `collectionCatchupTimeout` expires. This improves the user experience by preventing
	// the Access API from starting while the initial catchup completes. The intention is to avoid
	// returning NotFound errors immediately after startup for data that should be available, but has
	// not yet been indexed.
	// TODO (peter): consider removing this. I'm not convinced that this provides much value since in
	// the common case (node restart), the block finalization would also pause so the node should not
	// become farther behind in terms of collections.
	readyChan := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-readyChan:
		case <-time.After(s.collectionCatchupTimeout):
		}
		ready()
	}()

	requestCollectionsTicker := time.NewTicker(s.missingCollectionRequestInterval)
	defer requestCollectionsTicker.Stop()

	initialCatchupComplete := false
	for {
		err := s.requestMissingCollections(ctx, !initialCatchupComplete)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}

			ctx.Throw(fmt.Errorf("failed to request missing collections: %w", err))
			return
		}

		// after the first successful run, mark the catchup as complete. This will cause the worker
		// to call the ready function if it was not already called.
		if !initialCatchupComplete {
			initialCatchupComplete = true
			close(readyChan)
		}

		select {
		case <-requestCollectionsTicker.C:
		case <-ctx.Done():
			return
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

// requestMissingCollections requests all missing collections using local execution data if available,
// otherwise requests them from the network. Only sends requests to the network if we are more than
// `missingCollectionRequestThreshold` blocks behind the latest finalized block.
//
// Expected error returns during normal operation:
//   - [context.Canceled]: if the context is canceled before all collections are requested
func (s *Syncer) requestMissingCollections(ctx context.Context, forceSendRequests bool) error {
	lastFullBlockHeight := s.lastFullBlockHeight.Value()
	lastFinalizedBlock, err := s.state.Final().Head()
	if err != nil {
		return fmt.Errorf("failed to get finalized block: %w", err)
	}

	if lastFullBlockHeight >= lastFinalizedBlock.Height {
		return nil
	}

	// only send requests if we are sufficiently behind the latest finalized block to avoid spamming
	// collection nodes with requests.
	shouldSendRequestsToNetwork := forceSendRequests
	if lastFinalizedBlock.Height-lastFullBlockHeight >= s.missingCollectionRequestThreshold {
		shouldSendRequestsToNetwork = true
	}

	progress := util.LogProgress(s.log, util.DefaultLogProgressConfig("requesting missing collections", lastFinalizedBlock.Height-lastFullBlockHeight))

	requestedBlocks := 0
	requestedCollections := 0
	for height := lastFullBlockHeight + 1; height <= lastFinalizedBlock.Height; height++ {
		if ctx.Err() != nil {
			return fmt.Errorf("missing collection catchup interrupted: %w", ctx.Err())
		}

		collections, requested, err := s.requestForHeight(ctx, height, shouldSendRequestsToNetwork)
		if err != nil {
			return fmt.Errorf("failed to request collections for height %d: %w", height, err)
		}
		if requested {
			requestedBlocks++
			requestedCollections += len(collections)
		}

		progress(1)
	}

	if requestedBlocks > 0 {
		s.log.Warn().
			Uint64("finalized_height", lastFinalizedBlock.Height).
			Uint64("last_full_block_height", lastFullBlockHeight).
			Int("requested_block_count", requestedBlocks).
			Int("requested_collection_count", requestedCollections).
			Msg("re-requesting missing collections")
	}

	return nil
}

// requestForHeight requests all missing collections for the given height.
// If collections are available from execution data, they are indexed first. All other collections
// are requested from the network if `requestFromNetwork` is true.
// Returns true if the collections were missing and requested.
// Returns the list of collections that were requested from the network.
//
// No error returns are expected during normal operation.
func (s *Syncer) requestForHeight(ctx context.Context, height uint64, requestFromNetwork bool) ([]*flow.CollectionGuarantee, bool, error) {
	collections, err := s.indexer.MissingCollectionsAtHeight(height)
	if err != nil {
		return nil, false, fmt.Errorf("failed to find missing collections at height %d: %w", height, err)
	}
	if len(collections) == 0 {
		return nil, false, nil
	}

	// always index available collections from execution data.
	if s.execDataSyncer != nil {
		submitted, err := s.execDataSyncer.IndexForHeight(ctx, height)
		if err != nil {
			return nil, false, fmt.Errorf("failed to index collections from execution data: %w", err)
		}
		if submitted {
			return nil, true, nil
		}
	}

	// only request collections from the network if asked to do so.
	if !requestFromNetwork {
		return nil, false, nil
	}

	err = s.requestCollections(collections)
	if err != nil {
		return nil, false, fmt.Errorf("failed to request collections: %w", err)
	}
	// request made during catchup do not use Force() so they can be batched together for efficiency.
	// In practice, Force() is called once for each finalized block, so requests are dispatched at
	// least as often as the block rate.

	return collections, true, nil
}
