package extended

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jordanschalm/lockctx"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/access/systemcollection"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/storage"
)

const (
	// DefaultBackfillMaxWorkers is the maximum number of concurrent backfill workers. this may be multiplexed
	// across a larger number of backfilling indexers.
	DefaultBackfillMaxWorkers = 5

	// DefaultBackfillDelay is the delay between backfill attempts.
	DefaultBackfillDelay = 10 * time.Millisecond
)

type ExtendedIndexer struct {
	component.Component

	log             zerolog.Logger
	db              storage.DB
	metrics         module.ExtendedIndexingMetrics
	backfillDelay   time.Duration
	backfillWorkers int

	mu sync.RWMutex

	indexers         []Indexer
	liveIndexers     []Indexer
	backfillIndexers []Indexer

	startHeight  uint64
	latestHeight counters.StrictMonotonicCounter

	chainID           flow.ChainID
	systemCollections *access.Versioned[access.SystemCollectionBuilder]

	blocks      storage.Blocks
	collections storage.Collections
	events      storage.Events

	lockManager storage.LockManager
}

func NewExtendedIndexer(
	log zerolog.Logger,
	db storage.DB,
	indexers []Indexer,
	metrics module.ExtendedIndexingMetrics,
	backfillDelay time.Duration,
	backfillWorkers int,
	chainID flow.ChainID,
	blocks storage.Blocks,
	collections storage.Collections,
	events storage.Events,
	startHeight uint64,
	lockManager storage.LockManager,
) (*ExtendedIndexer, error) {
	if metrics == nil {
		// this is here mostly for anyone that imports this within an external package.
		return nil, fmt.Errorf("metrics cannot be nil. use a no-op metrics collector instead")
	}

	liveIndexers := make([]Indexer, 0)
	backfillIndexers := make([]Indexer, 0)
	for _, indexer := range indexers {
		latest, err := indexer.LatestIndexedHeight()
		if err != nil {
			return nil, fmt.Errorf("failed to get latest indexed height: %w", err)
		}
		if latest >= startHeight {
			liveIndexers = append(liveIndexers, indexer)
		} else {
			backfillIndexers = append(backfillIndexers, indexer)
		}
		metrics.InitializeLatestHeightExtended(indexer.Name(), latest)
	}

	c := &ExtendedIndexer{
		log:             log.With().Str("component", "extended_indexer").Logger(),
		db:              db,
		metrics:         metrics,
		backfillDelay:   backfillDelay,
		backfillWorkers: backfillWorkers,
		startHeight:     startHeight,
		latestHeight:    counters.NewMonotonicCounter(startHeight),

		indexers:         indexers,
		liveIndexers:     liveIndexers,
		backfillIndexers: backfillIndexers,

		chainID:           chainID,
		systemCollections: systemcollection.Default(chainID),

		blocks:      blocks,
		collections: collections,
		events:      events,

		lockManager: lockManager,
	}

	c.Component = component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()
			if err := c.Backfill(ctx); err != nil {
				ctx.Throw(fmt.Errorf("encountered error during backfill: %w", err))
			}
		}).
		Build()

	return c, nil
}

func (c *ExtendedIndexer) IndexBlockData(
	header *flow.Header,
	transactions []*flow.TransactionBody,
	events []flow.Event,
) error {
	latest := c.latestHeight.Value()
	if header.Height > latest+1 {
		return fmt.Errorf("cannot index future height %d (latest seen %d)", header.Height, latest)
	}
	// data for all blocks below the latest height is reindexed. individual indexers are responsible
	// for deduplicating past blocks.

	start := time.Now()

	eventsByTxIndex := make(map[uint32][]flow.Event)
	for _, event := range events {
		eventsByTxIndex[event.TransactionIndex] = append(eventsByTxIndex[event.TransactionIndex], event)
	}
	data := BlockData{
		Header:       header,
		Transactions: transactions,
		Events:       eventsByTxIndex,
	}

	batch := c.db.NewBatch()
	defer batch.Close()

	c.mu.RLock()
	liveIndexers := append([]Indexer(nil), c.liveIndexers...)
	c.mu.RUnlock()

	err := storage.WithLocks(c.lockManager, storage.LockGroupAccessIndexers, func(lctx lockctx.Context) error {
		return c.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			for _, indexer := range liveIndexers {
				if err := indexer.IndexBlockData(lctx, data, rw); err != nil {
					return fmt.Errorf("failed to index block data: %w", err)
				}
				c.metrics.BlockIndexedExtended(indexer.Name(), data.Header.Height)
			}
			return nil
		})
	})
	if err != nil {
		return fmt.Errorf("failed to index block data: %w", err)
	}

	if header.Height > latest {
		// double check to make sure it hasn't changed since we started indexing
		if !c.latestHeight.Set(header.Height) {
			return fmt.Errorf("failed to update latest height to %d, current %d", header.Height, latest)
		}
	}

	c.log.Debug().
		Dur("duration_ms", time.Since(start)).
		Int("indexer_count", len(c.indexers)).
		Uint64("block_height", header.Height).
		Msg("indexed account data")

	return nil
}

func (c *ExtendedIndexer) Backfill(ctx context.Context) error {
	c.mu.RLock()
	backfillIndexers := append([]Indexer(nil), c.backfillIndexers...)
	c.mu.RUnlock()

	if len(backfillIndexers) == 0 {
		return nil
	}

	// allow up to backfillMaxWorkers concurrent indexers to backfill at a time. used to limit the
	// db load from backfilling.
	workers := make(chan struct{}, c.backfillWorkers)
	g, gCtx := errgroup.WithContext(ctx)

	for _, indexer := range backfillIndexers {
		g.Go(func() error {
			progress, err := c.buildProgressFn(indexer)
			if err != nil {
				return fmt.Errorf("failed to build progress function: %w", err)
			}

			for {
				if gCtx.Err() != nil {
					return gCtx.Err()
				}

				liveProcessedHeight := c.latestHeight.Value()
				height, needsBackfill, err := nextBackfillHeight(indexer, liveProcessedHeight)
				if err != nil {
					return fmt.Errorf("failed to get next backfill height: %w", err)
				}
				if !needsBackfill {
					// indexer has caught up with the live group.
					c.promoteToLive(indexer)
					return nil
				}

				workers <- struct{}{}
				err = c.backfillHeight(height, indexer)
				<-workers
				if err != nil {
					// since we selected the next height based on the indexer's latest height, it is
					// unexpected that we receive any error from the indexer. This indicates concurrent
					// access which is not supported.
					return fmt.Errorf("failed to backfill height: %w", err)
				}

				c.metrics.BlockIndexedExtended(indexer.Name(), height)
				progress(1)

				// throttle per-indexer backfill loop
				pause(gCtx, c.backfillDelay)
			}
		})
	}

	return g.Wait()
}

func (c *ExtendedIndexer) buildProgressFn(indexer Indexer) (func(uint64), error) {
	latest, err := indexer.LatestIndexedHeight()
	if err != nil {
		return nil, fmt.Errorf("failed to get latest indexed height: %w", err)
	}
	targetHeight := c.latestHeight.Value()
	var remaining uint64
	if targetHeight > latest {
		remaining = targetHeight - latest
	}

	return util.LogProgress(c.log, util.DefaultLogProgressConfig(
		fmt.Sprintf("extended indexer backfill (%T)", indexer),
		remaining,
	)), nil
}

// nextBackfillHeight returns the next height to index based on persisted state.
// Invariant: we only attempt latest+1 for each indexer, so no heights are skipped.
func nextBackfillHeight(indexer Indexer, targetHeight uint64) (uint64, bool, error) {
	latest, err := indexer.LatestIndexedHeight()
	if err != nil {
		return 0, false, fmt.Errorf("failed to get latest indexed height: %w", err)
	}
	if latest >= targetHeight {
		return 0, false, nil
	}
	return latest + 1, true, nil
}

func (c *ExtendedIndexer) promoteToLive(indexer Indexer) {
	c.mu.Lock()
	defer c.mu.Unlock()

	remaining := make([]Indexer, 0, len(c.backfillIndexers))
	for _, candidate := range c.backfillIndexers {
		if candidate == indexer {
			continue
		}
		remaining = append(remaining, candidate)
	}
	c.backfillIndexers = remaining
	c.liveIndexers = append(c.liveIndexers, indexer)
}

// backfillHeight backfills the given height for the given indexer.
//
// Expected error returns during normal operation:
//   - [ErrAlreadyIndexed]: if the data is already indexed, skip to next height.
//   - [ErrFutureHeight]: if the data is for a future height, skip to next height.
func (c *ExtendedIndexer) backfillHeight(height uint64, indexer Indexer) error {
	blockData, err := c.blockData(height)
	if err != nil {
		return fmt.Errorf("failed to get block data: %w", err)
	}

	err = storage.WithLocks(c.lockManager, storage.LockGroupAccessIndexers, func(lctx lockctx.Context) error {
		return c.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return indexer.IndexBlockData(lctx, blockData, rw)
		})
	})
	if err != nil && !errors.Is(err, ErrAlreadyIndexed) {
		return fmt.Errorf("failed to index block data: %w", err)
	}

	return nil
}

// blockData loads the block data for the given height.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound]: if any data is not available for the height.
func (c *ExtendedIndexer) blockData(height uint64) (BlockData, error) {
	block, err := c.blocks.ByHeight(height)
	if err != nil {
		return BlockData{}, fmt.Errorf("failed to get header by height: %w", err)
	}

	events, err := c.events.ByBlockID(block.ID())
	if err != nil {
		return BlockData{}, fmt.Errorf("failed to get events by block id: %w", err)
	}

	eventsByTxIndex := make(map[uint32][]flow.Event)
	for _, event := range events {
		eventsByTxIndex[event.TransactionIndex] = append(eventsByTxIndex[event.TransactionIndex], event)
	}

	var transactions []*flow.TransactionBody
	for _, guarantee := range block.Payload.Guarantees {
		collection, err := c.collections.ByID(guarantee.CollectionID)
		if err != nil {
			return BlockData{}, fmt.Errorf("failed to get collection by id: %w", err)
		}
		transactions = append(transactions, collection.Transactions...)
	}

	sysCollection, err := c.systemCollections.
		ByHeight(block.Height).
		SystemCollection(c.chainID.Chain(), access.StaticEventProvider(events))
	if err != nil {
		return BlockData{}, fmt.Errorf("could not construct system collection: %w", err)
	}
	transactions = append(transactions, sysCollection.Transactions...)

	return BlockData{
		Header:       block.ToHeader(),
		Transactions: transactions,
		Events:       eventsByTxIndex,
	}, nil
}

func pause(ctx context.Context, duration time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(duration):
	}
}
