package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/dapperlabs/flow-go/model/flow"
)

// HotStuff Metrics
const (
	HotstuffEventTypeTimeout    = "timeout"
	HotstuffEventTypeOnProposal = "onproposal"
	HotstuffEventTypeOnVote     = "onvote"
)

// HotstuffCollector implements only the metrics emitted by the HotStuff core logic.
// We have multiple instances of HotStuff running within Flow: Consensus Nodes form
// the main consensus committee. In addition each Collector Node cluster runs their
// own HotStuff instance. Depending on the node role, the name space is different. Furthermore,
// even within the `collection` name space, we need to separate metrics between the different
// clusters. We do this by adding the label `committeeID` to the HotStuff metrics and
// allowing for configurable name space.
type HotstuffCollector struct {
	busyDuration    *prometheus.HistogramVec
	idleDuration    prometheus.Histogram
	waitDuration    *prometheus.HistogramVec
	curView         prometheus.Gauge
	qcView          prometheus.Gauge
	skips           prometheus.Counter
	timeouts        prometheus.Counter
	timeoutDuration prometheus.Gauge
}

func NewHotstuffCollector(chain flow.ChainID) *HotstuffCollector {

	hc := &HotstuffCollector{

		busyDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:        "busy_duration_seconds",
			Namespace:   namespaceConsensus,
			Subsystem:   subsystemHotstuff,
			Help:        "the duration of how long hotstuff's event loop has been busy processing one event",
			Buckets:     []float64{0.05, 0.2, 0.5, 1, 2, 5},
			ConstLabels: prometheus.Labels{LabelChain: chain.String()},
		}, []string{"event_type"}),

		idleDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:        "idle_duration_seconds",
			Namespace:   namespaceConsensus,
			Subsystem:   subsystemHotstuff,
			Help:        "the duration of how long hotstuff's event loop has been idle without processing any event",
			Buckets:     []float64{0.05, 0.2, 0.5, 1, 2, 5},
			ConstLabels: prometheus.Labels{LabelChain: chain.String()},
		}),

		waitDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:        "wait_duration_seconds",
			Namespace:   namespaceConsensus,
			Subsystem:   subsystemHotstuff,
			Help:        "the duration of how long an event has been waited in the hotstuff event loop queue before being processed.",
			Buckets:     []float64{0.05, 0.2, 0.5, 1, 2, 5},
			ConstLabels: prometheus.Labels{LabelChain: chain.String()},
		}, []string{"event_type"}),

		curView: promauto.NewGauge(prometheus.GaugeOpts{
			Name:        "cur_view",
			Namespace:   namespaceConsensus,
			Subsystem:   subsystemHotstuff,
			Help:        "the current view that the event handler has entered",
			ConstLabels: prometheus.Labels{LabelChain: chain.String()},
		}),

		qcView: promauto.NewGauge(prometheus.GaugeOpts{
			Name:        "qc_view",
			Namespace:   namespaceConsensus,
			Subsystem:   subsystemHotstuff,
			Help:        "The view of the newest known qc from hotstuff",
			ConstLabels: prometheus.Labels{LabelChain: chain.String()},
		}),

		skips: promauto.NewCounter(prometheus.CounterOpts{
			Name:        "skips_total",
			Namespace:   namespaceConsensus,
			Subsystem:   subsystemHotstuff,
			Help:        "The number of times we skipped ahead some views",
			ConstLabels: prometheus.Labels{LabelChain: chain.String()},
		}),

		timeouts: promauto.NewCounter(prometheus.CounterOpts{
			Name:        "timeouts_total",
			Namespace:   namespaceConsensus,
			Subsystem:   subsystemHotstuff,
			Help:        "The number of times we timed out during a view",
			ConstLabels: prometheus.Labels{LabelChain: chain.String()},
		}),

		timeoutDuration: promauto.NewGauge(prometheus.GaugeOpts{
			Name:        "timeout_seconds",
			Namespace:   namespaceConsensus,
			Subsystem:   subsystemHotstuff,
			Help:        "The current length of the timeout",
			ConstLabels: prometheus.Labels{LabelChain: chain.String()},
		}),
	}

	return hc
}

// HotStuffBusyDuration reports Metrics C6 HotStuff Busy Duration
func (hc *HotstuffCollector) HotStuffBusyDuration(duration time.Duration, event string) {
	hc.busyDuration.WithLabelValues(event).Observe(duration.Seconds())
}

// HotStuffIdleDuration reports Metrics C6 HotStuff Idle Duration
func (hc *HotstuffCollector) HotStuffIdleDuration(duration time.Duration) {
	hc.idleDuration.Observe(duration.Seconds())
}

// HotStuffWaitDuration reports Metrics C6 HotStuff Wait Duration
func (hc *HotstuffCollector) HotStuffWaitDuration(duration time.Duration, event string) {
	hc.waitDuration.WithLabelValues(event).Observe(duration.Seconds())
}

// HotstuffCollector reports Metrics C8: Current View
func (hc *HotstuffCollector) SetCurView(view uint64) {
	hc.curView.Set(float64(view))
}

// NewestKnownQC reports Metrics C9: View of Newest Known QC
func (hc *HotstuffCollector) SetQCView(view uint64) {
	hc.qcView.Set(float64(view))
}

// CountSkipped counts the number of skips we did.
func (hc *HotstuffCollector) CountSkipped() {
	hc.skips.Inc()
}

// CountTimeout counts the number of timeouts we had.
func (hc *HotstuffCollector) CountTimeout() {
	hc.timeouts.Inc()
}

// SetTimeout sets the current timeout duration.
func (hc *HotstuffCollector) SetTimeout(duration time.Duration) {
	hc.timeoutDuration.Set(duration.Seconds())
}
