package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// HotStuff Metrics
const (
	HotstuffEventTypeTimeout    = "timeout"
	HotstuffEventTypeOnProposal = "onproposal"
	HotstuffEventTypeOnVote     = "onvote"
)

// HotStuffMetrics implements only the metrics emitted by the HotStuff core logic.
// We have multiple instances of HotStuff running within Flow: Consensus Nodes form
// the main consensus committee. In addition each Collector Node cluster runs their
// own HotStuff instance. Depending on the node role, the name space is different. Furthermore,
// even within the `collection` name space, we need to separate metrics between the different
// clusters. We do this by adding the label `committeeID` to the HotStuff metrics and
// allowing for configurable name space.
type HotStuffMetrics struct {
	finalizedBlockCounter prometheus.Counter
	busyDuration          *prometheus.HistogramVec
	idleDuration          prometheus.Histogram
	waitDuration          *prometheus.HistogramVec
	newViewGauge          prometheus.Gauge
	newestKnownQcGauge    prometheus.Gauge
	blockProposalCounter  prometheus.Counter
}

func NewHotStuffMetrics(namespace string, committeeID string) *HotStuffMetrics {
	constLabels := map[string]string{"committee": committeeID}
	return &HotStuffMetrics{
		finalizedBlockCounter: promauto.NewCounter(prometheus.CounterOpts{
			Name:        "finalized_blocks",
			Namespace:   namespace,
			Subsystem:   subsystemHotStuff,
			Help:        "The number of finalized blocks",
			ConstLabels: constLabels,
		}),
		busyDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:        "busy_duration_seconds",
			Namespace:   namespace,
			Subsystem:   subsystemHotStuff,
			Help:        "the duration of how long hotstuff's event loop has been busy processing one event",
			Buckets:     []float64{0.05, 0.2, 0.5, 1, 2, 5},
			ConstLabels: constLabels,
		}, []string{"event_type"}),
		idleDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:        "idle_duration_seconds",
			Namespace:   namespace,
			Subsystem:   subsystemHotStuff,
			Help:        "the duration of how long hotstuff's event loop has been idle without processing any event",
			Buckets:     []float64{0.05, 0.2, 0.5, 1, 2, 5},
			ConstLabels: constLabels,
		}),
		waitDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:        "wait_duration_seconds",
			Namespace:   namespace,
			Subsystem:   subsystemHotStuff,
			Help:        "the duration of how long an event has been waited in the hotstuff event loop queue before being processed.",
			Buckets:     []float64{0.05, 0.2, 0.5, 1, 2, 5},
			ConstLabels: constLabels,
		}, []string{"event_type"}),
		newViewGauge: promauto.NewGauge(prometheus.GaugeOpts{
			Name:        "cur_view",
			Namespace:   namespace,
			Subsystem:   subsystemHotStuff,
			Help:        "the current view that the event handler has entered",
			ConstLabels: constLabels,
		}),
		newestKnownQcGauge: promauto.NewGauge(prometheus.GaugeOpts{
			Name:        "view_of_newest_known_qc",
			Namespace:   namespace,
			Subsystem:   subsystemHotStuff,
			Help:        "The view of the newest known qc from hotstuff",
			ConstLabels: constLabels,
		}),
		blockProposalCounter: promauto.NewCounter(prometheus.CounterOpts{
			Name:        "block_proposals",
			Namespace:   namespace,
			Subsystem:   subsystemHotStuff,
			Help:        "the number of block proposals made",
			ConstLabels: constLabels,
		}),
	}
}

// HotStuffBusyDuration reports Metrics C6 HotStuff Busy Duration
func (c *HotStuffMetrics) HotStuffBusyDuration(duration time.Duration, event string) {
	c.busyDuration.WithLabelValues(event).Observe(duration.Seconds())
}

// HotStuffIdleDuration reports Metrics C6 HotStuff Idle Duration
func (c *HotStuffMetrics) HotStuffIdleDuration(duration time.Duration) {
	c.idleDuration.Observe(duration.Seconds())
}

// HotStuffWaitDuration reports Metrics C6 HotStuff Wait Duration
func (c *HotStuffMetrics) HotStuffWaitDuration(duration time.Duration, event string) {
	c.waitDuration.WithLabelValues(event).Observe(duration.Seconds())
}

// FinalizedBlocks reports Metric C7: Number of Blocks Finalized (per second)
func (c *HotStuffMetrics) FinalizedBlocks(count int) {
	c.finalizedBlockCounter.Add(float64(count))
}

// HotStuffMetrics reports Metrics C8: Current View
func (c *HotStuffMetrics) StartNewView(view uint64) {
	c.newViewGauge.Set(float64(view))
}

// NewestKnownQC reports Metrics C9: View of Newest Known QC
func (c *HotStuffMetrics) NewestKnownQC(view uint64) {
	c.newestKnownQcGauge.Set(float64(view))
}

// MadeBlockProposal reports that a block proposal has been made
func (c *HotStuffMetrics) MadeBlockProposal() {
	c.blockProposalCounter.Inc()
}

type HotStuffNoopMetrics struct{}

// HotStuffBusyDuration reports Metrics C6 HotStuff Busy Duration
func (c *HotStuffNoopMetrics) HotStuffBusyDuration(time.Duration, string) {}

// HotStuffIdleDuration reports Metrics C6 HotStuff Idle Duration
func (c *HotStuffNoopMetrics) HotStuffIdleDuration(time.Duration) {}

// HotStuffWaitDuration reports Metrics C6 HotStuff Wait Duration
func (c *HotStuffNoopMetrics) HotStuffWaitDuration(time.Duration, string) {}

// FinalizedBlocks reports Metric C7: Number of Blocks Finalized (per second)
func (c *HotStuffNoopMetrics) FinalizedBlocks(int) {}

// HotStuffNoopMetrics reports Metrics C8: Current View
func (c *HotStuffNoopMetrics) StartNewView(uint64) {}

// NewestKnownQC reports Metrics C9: View of Newest Known QC
func (c *HotStuffNoopMetrics) NewestKnownQC(uint64) {}

// MadeBlockProposal reports that a block proposal has been made
func (c *HotStuffNoopMetrics) MadeBlockProposal() {}
