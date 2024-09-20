package metrics

import (
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

type collector struct {
	log zerolog.Logger

	collection chan metrics

	mu sync.Mutex

	lowestAvailableHeight uint64
	blocksAtHeight        map[uint64]map[flow.Identifier]struct{}
	metrics               map[flow.Identifier][]TransactionExecutionMetrics
}

func newCollector(
	log zerolog.Logger,
	lowestAvailableHeight uint64,
) *collector {
	return &collector{
		log:                   log,
		lowestAvailableHeight: lowestAvailableHeight,

		collection:     make(chan metrics, 1000),
		blocksAtHeight: make(map[uint64]map[flow.Identifier]struct{}),
		metrics:        make(map[flow.Identifier][]TransactionExecutionMetrics),
	}
}

// Collect should never block because it's called from the execution
func (c *collector) Collect(
	blockId flow.Identifier,
	blockHeight uint64,
	t TransactionExecutionMetrics,
) {
	select {
	case c.collection <- metrics{
		TransactionExecutionMetrics: t,
		blockHeight:                 blockHeight,
		blockId:                     blockId,
	}:
	default:
		c.log.Warn().
			Uint64("height", blockHeight).
			Msg("dropping metrics because the collection channel is full")
	}
}

func (c *collector) metricsCollectorWorker(
	ctx irrecoverable.SignalerContext,
	ready component.ReadyFunc,
) {
	ready()

	for {
		select {
		case <-ctx.Done():
			return
		case m := <-c.collection:
			c.collect(m.blockId, m.blockHeight, m.TransactionExecutionMetrics)
		}
	}
}

func (c *collector) collect(
	blockId flow.Identifier,
	blockHeight uint64,
	t TransactionExecutionMetrics,
) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if blockHeight <= c.lowestAvailableHeight {
		c.log.Warn().
			Uint64("height", blockHeight).
			Uint64("lowestAvailableHeight", c.lowestAvailableHeight).
			Msg("received metrics for a block that is older or equal than the most recent block")
		return
	}

	if _, ok := c.blocksAtHeight[blockHeight]; !ok {
		c.blocksAtHeight[blockHeight] = make(map[flow.Identifier]struct{})
	}
	c.blocksAtHeight[blockHeight][blockId] = struct{}{}
	c.metrics[blockId] = append(c.metrics[blockId], t)
}

// Pop returns the metrics for the given finalized block at the given height
// and clears all data up to the given height.
func (c *collector) Pop(height uint64, finalizedBlockId flow.Identifier) []TransactionExecutionMetrics {
	c.mu.Lock()
	defer c.mu.Unlock()

	if height <= c.lowestAvailableHeight {
		c.log.Warn().
			Uint64("height", height).
			Stringer("finalizedBlockId", finalizedBlockId).
			Msg("requested metrics for a finalizedBlockId that is older or equal than the most recent finalizedBlockId")
		return nil
	}

	// only return metrics for finalized block
	metrics := c.metrics[finalizedBlockId]

	c.advanceTo(height)

	return metrics
}

// advanceTo moves the latest height to the given height
// all data at lower heights will be deleted
func (c *collector) advanceTo(height uint64) {
	for c.lowestAvailableHeight < height {
		blocks := c.blocksAtHeight[c.lowestAvailableHeight]
		for block := range blocks {
			delete(c.metrics, block)
		}
		delete(c.blocksAtHeight, c.lowestAvailableHeight)
		c.lowestAvailableHeight++
	}
}
