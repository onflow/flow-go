package indexer

import (
	"errors"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/storage"
)

var _ module.CollectionExecutedMetric = (*CollectionExecutedMetricImpl)(nil)

// CollectionExecutedMetricImpl tracks metrics to measure how long it takes for tx to reach each step in their lifecycle
type CollectionExecutedMetricImpl struct {
	log zerolog.Logger // used to log relevant actions with context

	accessMetrics              module.AccessMetrics
	collectionsToMarkFinalized *stdmap.Times
	collectionsToMarkExecuted  *stdmap.Times
	blocksToMarkExecuted       *stdmap.Times

	collections storage.Collections
	blocks      storage.Blocks
}

func NewCollectionExecutedMetricImpl(
	log zerolog.Logger,
	accessMetrics module.AccessMetrics,
	collectionsToMarkFinalized *stdmap.Times,
	collectionsToMarkExecuted *stdmap.Times,
	blocksToMarkExecuted *stdmap.Times,
	collections storage.Collections,
	blocks storage.Blocks,
) (*CollectionExecutedMetricImpl, error) {
	return &CollectionExecutedMetricImpl{
		log:                        log,
		accessMetrics:              accessMetrics,
		collectionsToMarkFinalized: collectionsToMarkFinalized,
		collectionsToMarkExecuted:  collectionsToMarkExecuted,
		blocksToMarkExecuted:       blocksToMarkExecuted,
		collections:                collections,
		blocks:                     blocks,
	}, nil
}

// CollectionFinalized tracks collections to mark finalized
func (c *CollectionExecutedMetricImpl) CollectionFinalized(light flow.LightCollection) {
	if ti, found := c.collectionsToMarkFinalized.ByID(light.ID()); found {
		for _, t := range light.Transactions {
			c.accessMetrics.TransactionFinalized(t, ti)
		}
		c.collectionsToMarkFinalized.Remove(light.ID())
	}
}

// CollectionExecuted tracks collections to mark executed
func (c *CollectionExecutedMetricImpl) CollectionExecuted(light flow.LightCollection) {
	if ti, found := c.collectionsToMarkExecuted.ByID(light.ID()); found {
		for _, t := range light.Transactions {
			c.accessMetrics.TransactionExecuted(t, ti)
		}
		c.collectionsToMarkExecuted.Remove(light.ID())
	}
}

// BlockFinalized tracks finalized metric for block
func (c *CollectionExecutedMetricImpl) BlockFinalized(block *flow.Block) {
	// TODO: lookup actual finalization time by looking at the block finalizing `b`
	now := time.Now().UTC()
	blockID := block.ID()

	// mark all transactions as finalized
	// TODO: sample to reduce performance overhead
	for _, g := range block.Payload.Guarantees {
		l, err := c.collections.LightByID(g.CollectionID)
		if errors.Is(err, storage.ErrNotFound) {
			c.collectionsToMarkFinalized.Add(g.CollectionID, now)
			continue
		} else if err != nil {
			c.log.Warn().Err(err).Str("collection_id", g.CollectionID.String()).
				Msg("could not track tx finalized metric: finalized collection not found locally")
			continue
		}

		for _, t := range l.Transactions {
			c.accessMetrics.TransactionFinalized(t, now)
		}
	}

	if ti, found := c.blocksToMarkExecuted.ByID(blockID); found {
		c.blockExecuted(block, ti)
		c.accessMetrics.UpdateExecutionReceiptMaxHeight(block.Header.Height)
		c.blocksToMarkExecuted.Remove(blockID)
	}
}

// ExecutionReceiptReceived tracks execution receipt metrics
func (c *CollectionExecutedMetricImpl) ExecutionReceiptReceived(r *flow.ExecutionReceipt) {
	// TODO add actual execution time to execution receipt?
	now := time.Now().UTC()

	// retrieve the block
	// TODO: consider using storage.Index.ByBlockID, the index contains collection id and seals ID
	b, err := c.blocks.ByID(r.ExecutionResult.BlockID)

	if errors.Is(err, storage.ErrNotFound) {
		c.blocksToMarkExecuted.Add(r.ExecutionResult.BlockID, now)
		return
	}

	if err != nil {
		c.log.Warn().Err(err).Msg("could not track tx executed metric: executed block not found locally")
		return
	}

	c.accessMetrics.UpdateExecutionReceiptMaxHeight(b.Header.Height)

	c.blockExecuted(b, now)
}

func (c *CollectionExecutedMetricImpl) UpdateLastFullBlockHeight(height uint64) {
	c.accessMetrics.UpdateLastFullBlockHeight(height)
}

// blockExecuted tracks executed metric for block
func (c *CollectionExecutedMetricImpl) blockExecuted(block *flow.Block, ti time.Time) {
	// mark all transactions as executed
	// TODO: sample to reduce performance overhead
	for _, g := range block.Payload.Guarantees {
		l, err := c.collections.LightByID(g.CollectionID)
		if errors.Is(err, storage.ErrNotFound) {
			c.collectionsToMarkExecuted.Add(g.CollectionID, ti)
			continue
		} else if err != nil {
			c.log.Warn().Err(err).Str("collection_id", g.CollectionID.String()).
				Msg("could not track tx executed metric: executed collection not found locally")
			continue
		}

		for _, t := range l.Transactions {
			c.accessMetrics.TransactionExecuted(t, ti)
		}
	}
}
