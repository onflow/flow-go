package extended

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jordanschalm/lockctx"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/access/systemcollection"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

const (
	// DefaultBackfillDelay defines the delay between consecutive backfill attempts.
	// Set carefully to avoid overwhelming the system.
	// With the default value of 5ms, the maximum catch-up rate is approximately
	// 200 blocks per second.
	DefaultBackfillDelay = 5 * time.Millisecond
)

// ExtendedIndexer orchestrates indexing for all extended indexers.
//
// Indexing is performed in a single-threaded loop, where each iteration indexes the next height for
// all indexers. Indexers are grouped by their next height to reduce database lookups. All data for
// each height is written to a batch and committed at once.
//
// NOT CONCURRENCY SAFE.
type ExtendedIndexer struct {
	component.Component

	log           zerolog.Logger
	db            storage.DB
	lockManager   storage.LockManager
	metrics       module.ExtendedIndexingMetrics
	backfillDelay time.Duration

	chainID     flow.ChainID
	state       protocol.State
	headers     storage.Headers
	index       storage.Index
	collections storage.Collections
	guarantees  storage.Guarantees
	events      storage.Events
	results     storage.LightTransactionResults

	systemCollections *access.Versioned[access.SystemCollectionBuilder]

	indexers []Indexer
	notifier engine.Notifier

	// mu protects the latestBlockData field which is written in another goroutine via IndexBlockData.
	mu sync.RWMutex

	// latestBlockData is the latest block data received via IndexBlockData.
	// This represents the "live" block data that is being indexed.
	latestBlockData *BlockData
}

var _ IndexerManager = (*ExtendedIndexer)(nil)

func NewExtendedIndexer(
	log zerolog.Logger,
	metrics module.ExtendedIndexingMetrics,
	db storage.DB,
	lockManager storage.LockManager,
	state protocol.State,
	index storage.Index,
	headers storage.Headers,
	guarantees storage.Guarantees,
	collections storage.Collections,
	events storage.Events,
	results storage.LightTransactionResults,
	indexers []Indexer,
	chainID flow.ChainID,
	backfillDelay time.Duration,
) (*ExtendedIndexer, error) {
	if metrics == nil {
		// this is here mostly for anyone that imports this within an external package.
		return nil, fmt.Errorf("metrics cannot be nil. use a no-op metrics collector instead")
	}

	log = log.With().Str("component", "extended_indexer").Logger()
	c := &ExtendedIndexer{
		log:           log,
		db:            db,
		lockManager:   lockManager,
		metrics:       metrics,
		backfillDelay: backfillDelay,
		indexers:      indexers,
		notifier:      engine.NewNotifier(),

		chainID:           chainID,
		state:             state,
		headers:           headers,
		index:             index,
		guarantees:        guarantees,
		collections:       collections,
		events:            events,
		results:           results,
		systemCollections: systemcollection.Default(chainID),
	}

	c.Component = component.NewComponentManagerBuilder().
		AddWorker(c.ingestLoop).
		Build()

	return c, nil
}

// IndexBlockExecutionData captures the block data and makes it available to the indexers.
//
// No error returns are expected during normal operation.
func (c *ExtendedIndexer) IndexBlockExecutionData(
	data *execution_data.BlockExecutionDataEntity,
) error {
	header, err := c.state.AtBlockID(data.BlockID).Head()
	if err != nil {
		return fmt.Errorf("failed to get block by id: %w", err)
	}

	txs, events, err := c.extractDataFromExecutionData(data)
	if err != nil {
		return fmt.Errorf("failed to extract data from execution data: %w", err)
	}

	return c.IndexBlockData(header, txs, events)
}

// IndexBlockData stores block data and exposes it to the indexers.
// It must be called sequentially, with blocks provided in strictly
// increasing height order.
//
// Typically, this method is invoked when the latest block is received.
// If the indexer is fully caught up, this latest block will be the next
// one to process, and indexing it will advance the indexed height.
//
// If the indexer is still catching up, however, the latest block is not
// immediately needed because the indexer must first process older blocks.
//
// For this reason, we do not index the latest block right away. Instead,
// we cache it and notify the worker to proceed with the next job.
//
// If the next job is to process the latest block, the cached
// c.latestBlockData will be used. Otherwise, if the job is to process
// older blocks, the cache is ignored and the worker fetches the required
// block data for indexing.
//
// No error returns are expected during normal operation.
func (c *ExtendedIndexer) IndexBlockData(
	header *flow.Header,
	transactions []*flow.TransactionBody,
	events []flow.Event,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.latestBlockData != nil {
		if header.Height > c.latestBlockData.Header.Height+1 {
			return fmt.Errorf("unexpected block received: expected height %d, got %d", c.latestBlockData.Header.Height+1, header.Height)
		}
		if header.Height <= c.latestBlockData.Header.Height {
			return nil
		}
	}

	c.latestBlockData = &BlockData{
		Header:       header,
		Transactions: transactions,
		Events:       groupEventsByTxIndex(events),
	}
	c.notifier.Notify()

	return nil
}

// ingestLoop is the main ingestion loop for the extended indexer.
// It indexes the next heights for all indexers, and handles backfilling from storage.
//
// NOT CONCURRENCY SAFE! Only one instance may be run at a time.
func (c *ExtendedIndexer) ingestLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	timer := time.NewTimer(c.backfillDelay)
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.notifier.Channel():
		case <-timer.C:
		}

		hasBackfillingIndexers, err := c.indexNextHeights()
		if err != nil {
			ctx.Throw(fmt.Errorf("failed to check all: %w", err))
			return
		}

		// once all indexers are caught up with the live height, stop resetting the backfill timer
		// so the only notification will be for new live blocks.
		if hasBackfillingIndexers {
			timer.Reset(c.backfillDelay)
		}
	}
}

// indexNextHeights indexes the next heights for all indexers.
// This is the main indexing method which handles processing for all configured indexer. On each call,
// it passes data for the next height to each indexer, and stores data from all indexers in a single batch.
// Indexers are not required to be at the same height.
//
// NOT CONCURRENCY SAFE.
//
// No error returns are expected during normal operation.
func (c *ExtendedIndexer) indexNextHeights() (bool, error) {
	c.mu.RLock()
	latestBlockData := c.latestBlockData
	c.mu.RUnlock()

	// group indexers by their next height to allow the indexers to share input data.
	liveGroup, backfillGroups, err := buildGroupLookup(c.indexers, latestBlockData)
	if err != nil {
		return false, fmt.Errorf("failed to build group lookup: %w", err)
	}

	if len(liveGroup) > 0 {
		err = c.runIndexers(liveGroup, latestBlockData)
		if err != nil {
			return false, fmt.Errorf("failed to index live indexers: %w", err)
		}
	}

	// this is a trailing indicator. this will be true if any indexer was backfilled in this iteration.
	// if all indexers catch up, it will take one more iteration to register as all caught up.
	hasBackfillingIndexers := len(backfillGroups) > 0

	for height, group := range backfillGroups {
		data, err := c.blockDataFromStorage(height)
		if err != nil {
			if !errors.Is(err, storage.ErrNotFound) {
				return false, fmt.Errorf("failed to get block data for height %d: %w", height, err)
			}

			// on startup, it's possible that indexers are already caught up, but the live block has
			// not been provided yet. in this case, skip the group until the live block is known.
			// this check will ensure backfilling progresses if and only if the live block is also
			// progressing.
			if latestBlockData == nil {
				continue
			}

			// if the live block is known, it must have all data available since this height is below
			// the `latestBlockData` height. otherwise the database is in an inconsistent state.
			return false, fmt.Errorf("failed to get block data for height %d when latest block %d is known: %w",
				height, latestBlockData.Header.Height, err)
		}

		err = c.runIndexers(group, &data)
		if err != nil {
			return false, fmt.Errorf("failed to index backfill indexers: %w", err)
		}
	}

	return hasBackfillingIndexers, nil
}

// runIndexers indexes the data for all indexers in the group.
//
// No error returns are expected during normal operation.
func (c *ExtendedIndexer) runIndexers(indexers []Indexer, data *BlockData) error {
	height := data.Header.Height
	return storage.WithLocks(c.lockManager, storage.LockGroupAccessExtendedIndexers, func(lctx lockctx.Context) error {
		return c.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			// index the data for all indexers in the group
			for _, indexer := range indexers {
				if err := indexer.IndexBlockData(lctx, *data, rw); err != nil {
					if errors.Is(err, ErrAlreadyIndexed) {
						continue
					}
					// ErrFutureHeight is not expected since we have already checked that `data`'s height
					// is the next height. If it is not, there is a bug.
					return fmt.Errorf("failed to index block data for %s at height %d: %w", indexer.Name(), height, err)
				}

				c.metrics.BlockIndexedExtended(indexer.Name(), height)
			}

			return nil
		})
	})
}

// blockDataFromStorage loads the block data for the given height.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound]: if any data is not available for the height.
func (c *ExtendedIndexer) blockDataFromStorage(height uint64) (BlockData, error) {
	// special handling for the spork root block which has no transactions or events.
	if height == c.state.Params().SporkRootBlockHeight() {
		return BlockData{
			Header: c.state.Params().SporkRootBlock().ToHeader(),
		}, nil
	}

	blockID, err := c.headers.BlockIDByHeight(height)
	if err != nil {
		return BlockData{}, fmt.Errorf("failed to get block id by height: %w", err)
	}

	header, err := c.headers.ByBlockID(blockID)
	if err != nil {
		return BlockData{}, fmt.Errorf("failed to get header by id: %w", err)
	}

	// all we need are the guarantees for the block, so use the index to avoid loading unnecessary
	// execution payload data.
	blockIndex, err := c.index.ByBlockID(blockID)
	if err != nil {
		return BlockData{}, fmt.Errorf("failed to get block index by block id: %w", err)
	}

	events, err := c.events.ByBlockID(blockID)
	if err != nil {
		return BlockData{}, fmt.Errorf("failed to get events by block id: %w", err)
	}

	// getting events returns an empty slice and no error if no events are found. This could mean
	// either that the block has no events, or that they have not been indexed yet. In this case, also
	// check if there were any transaction results. All blocks should have at least one system tx.
	// if not, then assume the block is not indexed yet.
	// Note: we need to check both because it's possible the system transaction failed and did not
	// produce any events. It is very uncommon for a block to have no events, so this logic will
	// rarely be run.
	if len(events) == 0 {
		results, err := c.results.ByBlockID(blockID)
		if err != nil {
			return BlockData{}, fmt.Errorf("failed to get results by block id: %w", err)
		}

		// results will similarly return an empty slice and no error if no results are found
		// or if the block's execution data is not indexed yet.
		if len(results) == 0 {
			return BlockData{}, fmt.Errorf("results for block %d not indexed yet: %w", height, storage.ErrNotFound)
		}
	}

	var transactions []*flow.TransactionBody
	for _, guaranteeID := range blockIndex.GuaranteeIDs {
		guarantee, err := c.guarantees.ByID(guaranteeID)
		if err != nil {
			return BlockData{}, fmt.Errorf("failed to get guarantee by id: %w", err)
		}
		collection, err := c.collections.ByID(guarantee.CollectionID)
		if err != nil {
			return BlockData{}, fmt.Errorf("failed to get collection by id: %w", err)
		}
		transactions = append(transactions, collection.Transactions...)
	}

	// the system collection is not indexed, so construct it.
	sysCollection, err := c.systemCollections.
		ByHeight(height).
		SystemCollection(c.chainID.Chain(), access.StaticEventProvider(events))
	if err != nil {
		return BlockData{}, fmt.Errorf("could not construct system collection: %w", err)
	}
	transactions = append(transactions, sysCollection.Transactions...)

	return BlockData{
		Header:       header,
		Transactions: transactions,
		Events:       groupEventsByTxIndex(events),
	}, nil
}

// extractDataFromExecutionData extracts the transaction and event data from the execution data.
//
// No error returns are expected during normal operation.
func (c *ExtendedIndexer) extractDataFromExecutionData(data *execution_data.BlockExecutionDataEntity) ([]*flow.TransactionBody, []flow.Event, error) {
	txs := make([]*flow.TransactionBody, 0)
	events := make([]flow.Event, 0)
	for i, chunk := range data.ChunkExecutionDatas {
		// block execution data should include collections for ALL chunks, including the system chunk.
		if chunk.Collection == nil {
			return nil, nil, fmt.Errorf("chunk %d collection is nil", i)
		}
		txs = append(txs, chunk.Collection.Transactions...)
		events = append(events, chunk.Events...)
	}
	return txs, events, nil
}

// buildGroupLookup builds a map of indexers by their next height to index.
// This allows the indexing loop to lookup data for a height once, and pass it to all indexers in the group.
// If `latestBlockData` is not nil, it will also return the group of indexers at the "live" height.
// All indexers that are ahead of the live block will be skipped.
//
// No error returns are expected during normal operation.
func buildGroupLookup(indexers []Indexer, latestBlockData *BlockData) ([]Indexer, map[uint64][]Indexer, error) {
	backfillGroups := make(map[uint64][]Indexer)
	liveGroup := make([]Indexer, 0)
	for _, indexer := range indexers {
		nextHeight, err := indexer.NextHeight()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get next height for indexer %s: %w", indexer.Name(), err)
		}
		if latestBlockData != nil {
			if nextHeight > latestBlockData.Header.Height {
				continue // skip all indexers that are ahead of the live block
			}
			if nextHeight == latestBlockData.Header.Height {
				liveGroup = append(liveGroup, indexer)
				continue
			}
		}
		backfillGroups[nextHeight] = append(backfillGroups[nextHeight], indexer)
	}

	return liveGroup, backfillGroups, nil
}

// groupEventsByTxIndex returns a map of events grouped by transaction index in the original event order.
func groupEventsByTxIndex(events []flow.Event) map[uint32][]flow.Event {
	eventsByTxIndex := make(map[uint32][]flow.Event)
	for _, event := range events {
		eventsByTxIndex[event.TransactionIndex] = append(eventsByTxIndex[event.TransactionIndex], event)
	}
	return eventsByTxIndex
}
