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
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/storage"
)

const (
	// DefaultBackfillDelay is the delay between backfill attempts.
	DefaultBackfillDelay = 10 * time.Millisecond
)

// ExtendedIndexer orchestrates indexing for all extended indexers.
//
// Indexing is performed in a single-threaded loop, where each iteration indexes the next height for
// all indexers. Indexers are grouped by their next height to reduce database lookups. All data for
// each iteration is written to a batch and committed at once.
//
// NOT CONCURRENCY SAFE.
type ExtendedIndexer struct {
	component.Component

	log           zerolog.Logger
	db            storage.DB
	lockManager   storage.LockManager
	metrics       module.ExtendedIndexingMetrics
	backfillDelay time.Duration

	chainID           flow.ChainID
	blocks            storage.Blocks
	collections       storage.Collections
	events            storage.Events
	systemCollections *access.Versioned[access.SystemCollectionBuilder]

	indexers        []Indexer
	groupLookup     map[uint64][]Indexer
	notifier        engine.Notifier
	progressManager *progressManager

	mu              sync.RWMutex
	latestBlockData *BlockData
}

func NewExtendedIndexer(
	log zerolog.Logger,
	metrics module.ExtendedIndexingMetrics,
	db storage.DB,
	lockManager storage.LockManager,
	blocks storage.Blocks,
	collections storage.Collections,
	events storage.Events,
	indexers []Indexer,
	chainID flow.ChainID,
	backfillDelay time.Duration,
) (*ExtendedIndexer, error) {
	if metrics == nil {
		// this is here mostly for anyone that imports this within an external package.
		return nil, fmt.Errorf("metrics cannot be nil. use a no-op metrics collector instead")
	}

	groupLookup, err := buildGroupLookup(indexers)
	if err != nil {
		return nil, fmt.Errorf("failed to build group lookup: %w", err)
	}

	log = log.With().Str("component", "extended_indexer").Logger()
	c := &ExtendedIndexer{
		log:           log,
		db:            db,
		lockManager:   lockManager,
		metrics:       metrics,
		backfillDelay: backfillDelay,

		indexers:        indexers,
		groupLookup:     groupLookup,
		notifier:        engine.NewNotifier(),
		progressManager: newProgressManager(log),

		chainID:           chainID,
		blocks:            blocks,
		collections:       collections,
		events:            events,
		systemCollections: systemcollection.Default(chainID),
	}

	c.Component = component.NewComponentManagerBuilder().
		AddWorker(c.ingestLoop).
		Build()

	return c, nil
}

// IndexBlockData captures the block data and makes it available to the indexers.
//
// No error returns are expected during normal operation.
func (c *ExtendedIndexer) IndexBlockData(
	header *flow.Header,
	transactions []*flow.TransactionBody,
	events []flow.Event,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.latestBlockData != nil && header.Height != c.latestBlockData.Header.Height+1 {
		return fmt.Errorf("expected height %d, but got %d", c.latestBlockData.Header.Height+1, header.Height)
	}

	eventsByTxIndex := make(map[uint32][]flow.Event)
	for _, event := range events {
		eventsByTxIndex[event.TransactionIndex] = append(eventsByTxIndex[event.TransactionIndex], event)
	}

	c.latestBlockData = &BlockData{
		Header:       header,
		Transactions: transactions,
		Events:       eventsByTxIndex,
	}
	c.notifier.Notify()

	return nil
}

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
		// TODO: do we need to enforce a minimum delay?

		if err := c.indexNextHeights(); err != nil {
			ctx.Throw(fmt.Errorf("failed to check all: %w", err))
			return
		}

		if c.hasBackfillingIndexers() {
			timer.Reset(c.backfillDelay)
		}
	}
}

func (c *ExtendedIndexer) hasBackfillingIndexers() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.latestBlockData == nil {
		return true
	}

	liveHeight := c.latestBlockData.Header.Height + 1
	for height := range c.groupLookup {
		if height < liveHeight {
			return true
		}
	}
	return false
}

// indexNextHeights indexes the next heights for all indexers.
// This is the main indexing method which handles processing for all configured indexer. On each call,
// it passes data for the next height to each indexer, and stores data from all indexers in a single batch.
// Indexers are not required to be at the same height.
//
// NOT CONCURRENCY SAFE.
//
// No error returns are expected during normal operation.
func (c *ExtendedIndexer) indexNextHeights() error {
	c.mu.RLock()
	latestBlockData := c.latestBlockData
	c.mu.RUnlock()

	err := storage.WithLocks(c.lockManager, storage.LockGroupAccessExtendedIndexers, func(lctx lockctx.Context) error {
		return c.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			for height, group := range c.groupLookup {
				var data BlockData
				var err error

				// get the data for the height
				// latestBlockData may be nil if the indexer just started. in this case, fall back to fetching data
				// from storage. If the node has not indexed any data yet, its possible storage will also be missing
				// the data.
				if latestBlockData != nil && height == latestBlockData.Header.Height {
					data = *latestBlockData
				} else {
					data, err = c.blockData(height)
					if err != nil {
						if errors.Is(err, storage.ErrNotFound) {
							continue // skip group for this iteration
						}
						return fmt.Errorf("failed to get block data for height %d: %w", height, err)
					}
				}

				// index the data for all indexers in the group
				for _, indexer := range group {
					if err := indexer.IndexBlockData(lctx, data, rw); err != nil {
						if errors.Is(err, ErrAlreadyIndexed) {
							continue
						}
						return fmt.Errorf("failed to index block data for %s at height %d: %w", indexer.Name(), height, err)
					}

					c.metrics.BlockIndexedExtended(indexer.Name(), height)
					if latestBlockData != nil {
						// we will skip logging progress until data for the first live block is received.
						c.progressManager.track(indexer.Name(), height, latestBlockData.Header.Height)
					}
				}
			}
			return nil
		})
	})
	if err != nil {
		return fmt.Errorf("failed to index block data: %w", err)
	}

	// after the batch is committed, get the new next height for each indexer and refresh groups
	newGroups, err := buildGroupLookup(c.indexers)
	if err != nil {
		return fmt.Errorf("failed to build group lookup: %w", err)
	}
	c.groupLookup = newGroups

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

func buildGroupLookup(indexers []Indexer) (map[uint64][]Indexer, error) {
	groupLookup := make(map[uint64][]Indexer)
	for _, indexer := range indexers {
		nextHeight, err := indexer.NextHeight()
		if err != nil {
			return nil, fmt.Errorf("failed to get next height for indexer %s: %w", indexer.Name(), err)
		}
		groupLookup[nextHeight] = append(groupLookup[nextHeight], indexer)
	}
	return groupLookup, nil
}

type progressTracker struct {
	progressFn   func(uint64)
	targetHeight uint64
}

func (t *progressTracker) track(height uint64) {
	if height <= t.targetHeight {
		t.progressFn(1)
	}
}

type progressManager struct {
	log      zerolog.Logger
	trackers map[string]*progressTracker
}

func newProgressManager(log zerolog.Logger) *progressManager {
	return &progressManager{
		log:      log,
		trackers: make(map[string]*progressTracker),
	}
}

func (m *progressManager) track(name string, height, targetHeight uint64) {
	tracker, ok := m.trackers[name]
	if !ok {
		if height >= targetHeight {
			return
		}

		tracker = &progressTracker{
			targetHeight: targetHeight,
			progressFn: util.LogProgress(m.log, util.DefaultLogProgressConfig(
				fmt.Sprintf("extended indexer backfill (%s)", name),
				targetHeight-height,
			)),
		}
		m.trackers[name] = tracker
	}

	tracker.track(height)
}
