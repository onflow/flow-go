package synchronization

import (
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/onflow/flow-go/model/flow"
)

type SynchronizationMetrics interface {
	// record pruned blocks. requested and received times might be zero values
	PrunedBlockById(status *Status)

	PrunedBlockByHeight(status *Status)

	// totalByHeight and totalById are the number of blocks pruned for blocks requested by height and by id
	// storedByHeight and storedById are the number of blocks still stored by height and id
	PrunedBlocks(totalByHeight, totalById, storedByHeight, storedById int)

	RangeRequested(ran flow.Range)

	BatchRequested(batch flow.Batch)
}

type NoopMetrics struct{}

func (nc *NoopMetrics) PrunedBlockById(status *Status)                                        {}
func (nc *NoopMetrics) PrunedBlockByHeight(status *Status)                                    {}
func (nc *NoopMetrics) PrunedBlocks(totalByHeight, totalById, storedByHeight, storedById int) {}
func (nc *NoopMetrics) RangeRequested(ran flow.Range)                                         {}
func (nc *NoopMetrics) BatchRequested(batch flow.Batch)                                       {}

const (
	namespaceSynchronization = "synchronization"
	subsystemSyncCore        = "sync_core"
)

type MetricsCollector struct {
	timeToPruned          *prometheus.HistogramVec
	timeToReceived        *prometheus.HistogramVec
	totalPruned           *prometheus.CounterVec
	storedByHeight        prometheus.Gauge
	storedById            prometheus.Gauge
	totalHeightsRequested prometheus.Counter
	totalIdsRequested     prometheus.Counter
}

func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		timeToPruned: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:      "time_to_pruned",
			Namespace: namespaceSynchronization,
			Subsystem: subsystemSyncCore,
			Help:      "the time between queueing and pruning a block in milliseconds",
			Buckets:   []float64{100, 250, 500, 1000, 2500, 5000, 7500, 10000, 20000},
		}, []string{"status", "requested_by"}),
		timeToReceived: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:      "time_to_received",
			Namespace: namespaceSynchronization,
			Subsystem: subsystemSyncCore,
			Help:      "the time between queueing and receiving a block in milliseconds",
			Buckets:   []float64{100, 250, 500, 1000, 2500, 5000, 7500, 10000, 20000},
		}, []string{"requested_by"}),
		totalPruned: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name:      "total_blocks_pruned",
			Namespace: namespaceSynchronization,
			Subsystem: subsystemSyncCore,
			Help:      "the total number of blocks pruned by 'id' or 'height'",
		}, []string{"requested_by"}),
		storedByHeight: prometheus.NewGauge(prometheus.GaugeOpts{
			Name:      "blocks_stored_by_height",
			Namespace: namespaceSynchronization,
			Subsystem: subsystemSyncCore,
			Help:      "the number of blocks currently stored by height",
		}),
		storedById: prometheus.NewGauge(prometheus.GaugeOpts{
			Name:      "blocks_stored_by_id",
			Namespace: namespaceSynchronization,
			Subsystem: subsystemSyncCore,
			Help:      "the number of blocks currently stored by id",
		}),
		totalHeightsRequested: prometheus.NewCounter(prometheus.CounterOpts{
			Name:      "total_heights_requested",
			Namespace: namespaceSynchronization,
			Subsystem: subsystemSyncCore,
			Help:      "the total number of blocks requested by height. range of 0-10 would increase the counter by 10",
		}),
		totalIdsRequested: prometheus.NewCounter(prometheus.CounterOpts{
			Name:      "total_ids_requested",
			Namespace: namespaceSynchronization,
			Subsystem: subsystemSyncCore,
			Help:      "the total number of blocks requested by id",
		}),
	}
}

func (s *MetricsCollector) PrunedBlockById(status *Status) {
	s.prunedBlock(status, "id")
}

func (s *MetricsCollector) PrunedBlockByHeight(status *Status) {
	s.prunedBlock(status, "height")
}

func (s *MetricsCollector) prunedBlock(status *Status, requestedBy string) {
	str := strings.ToLower(status.StatusString())

	// measure the time-to-pruned
	pruned := float64(time.Since(status.Queued).Milliseconds())
	s.timeToPruned.With(prometheus.Labels{"status": str, "requested_by": requestedBy}).Observe(pruned)

	if status.WasReceived() {
		// measure the time-to-received
		received := float64(status.Received.Sub(status.Queued).Milliseconds())
		s.timeToReceived.With(prometheus.Labels{"requested_by": requestedBy}).Observe(received)
	}
}

func (s *MetricsCollector) PrunedBlocks(totalByHeight, totalById, storedByHeight, storedById int) {
	// add the total number of blocks pruned
	s.totalPruned.With(prometheus.Labels{"requested_by": "id"}).Add(float64(totalById))
	s.totalPruned.With(prometheus.Labels{"requested_by": "height"}).Add(float64(totalByHeight))

	// update gauges
	s.storedById.Set(float64(storedById))
	s.storedByHeight.Set(float64(storedByHeight))

}

func (s *MetricsCollector) RangeRequested(ran flow.Range) {
	s.totalHeightsRequested.Add(float64(ran.To - ran.From))
}

func (s *MetricsCollector) BatchRequested(batch flow.Batch) {
	s.totalIdsRequested.Add(float64(len(batch.BlockIDs)))
}
