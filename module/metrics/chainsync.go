package metrics

import (
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/onflow/flow-go/model/chainsync"
	"github.com/onflow/flow-go/model/flow"
)

type ChainSyncCollector struct {
	chainID               flow.ChainID
	timeToPruned          *prometheus.HistogramVec
	timeToReceived        *prometheus.HistogramVec
	totalPruned           *prometheus.CounterVec
	storedBlocks          *prometheus.GaugeVec
	totalHeightsRequested prometheus.Counter
	totalIdsRequested     prometheus.Counter
}

func NewChainSyncCollector(chainID flow.ChainID) *ChainSyncCollector {
	return &ChainSyncCollector{
		chainID: chainID,
		timeToPruned: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:      "time_to_pruned_seconds",
			Namespace: namespaceChainsync,
			Subsystem: subsystemSyncCore,
			Help:      "the time between queueing and pruning a block in seconds",
			Buckets:   []float64{.1, .25, .5, 1, 2.5, 5, 7.5, 10, 20},
		}, []string{"status", "requested_by"}),
		timeToReceived: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:      "time_to_received_seconds",
			Namespace: namespaceChainsync,
			Subsystem: subsystemSyncCore,
			Help:      "the time between queueing and receiving a block in seconds",
			Buckets:   []float64{.1, .25, .5, 1, 2.5, 5, 7.5, 10, 20},
		}, []string{"requested_by"}),
		totalPruned: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name:      "blocks_pruned_total",
			Namespace: namespaceChainsync,
			Subsystem: subsystemSyncCore,
			Help:      "the total number of blocks pruned by 'id' or 'height'",
		}, []string{"requested_by"}),
		storedBlocks: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name:      "blocks_stored_total",
			Namespace: namespaceChainsync,
			Subsystem: subsystemSyncCore,
			Help:      "the total number of blocks currently stored by 'id' or 'height'",
		}, []string{"requested_by"}),
		totalHeightsRequested: prometheus.NewCounter(prometheus.CounterOpts{
			Name:      "block_heights_requested_total",
			Namespace: namespaceChainsync,
			Subsystem: subsystemSyncCore,
			Help:      "the total number of blocks requested by height, including retried requests for the same heights. Eg: a range of 1-10 would increase the counter by 10",
		}),
		totalIdsRequested: prometheus.NewCounter(prometheus.CounterOpts{
			Name:      "block_ids_requested_total",
			Namespace: namespaceChainsync,
			Subsystem: subsystemSyncCore,
			Help:      "the total number of blocks requested by id",
		}),
	}
}

func (c *ChainSyncCollector) PrunedBlockById(status *chainsync.Status) {
	c.prunedBlock(status, "id")
}

func (c *ChainSyncCollector) PrunedBlockByHeight(status *chainsync.Status) {
	c.prunedBlock(status, "height")
}

func (c *ChainSyncCollector) prunedBlock(status *chainsync.Status, requestedBy string) {
	str := strings.ToLower(status.StatusString())

	// measure the time-to-pruned
	pruned := float64(time.Since(status.Queued).Milliseconds())
	c.timeToPruned.With(prometheus.Labels{"status": str, "requested_by": requestedBy}).Observe(pruned)

	if status.WasReceived() {
		// measure the time-to-received
		received := float64(status.Received.Sub(status.Queued).Milliseconds())
		c.timeToReceived.With(prometheus.Labels{"requested_by": requestedBy}).Observe(received)
	}
}

func (c *ChainSyncCollector) PrunedBlocks(totalByHeight, totalById, storedByHeight, storedById int) {
	// add the total number of blocks pruned
	c.totalPruned.With(prometheus.Labels{"requested_by": "id"}).Add(float64(totalById))
	c.totalPruned.With(prometheus.Labels{"requested_by": "height"}).Add(float64(totalByHeight))

	// update gauges
	c.storedBlocks.With(prometheus.Labels{"requested_by": "id"}).Set(float64(storedById))
	c.storedBlocks.With(prometheus.Labels{"requested_by": "height"}).Set(float64(storedByHeight))
}

func (c *ChainSyncCollector) RangeRequested(ran chainsync.Range) {
	c.totalHeightsRequested.Add(float64(ran.Len()))
}

func (c *ChainSyncCollector) BatchRequested(batch chainsync.Batch) {
	c.totalIdsRequested.Add(float64(len(batch.BlockIDs)))
}
