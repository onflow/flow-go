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
// the main consensus committee. In addition each Collector node cluster runs their
// own HotStuff instance. Depending on the node role, the name space is different. Furthermore,
// even within the `collection` name space, we need to separate metrics between the different
// clusters. We do this by adding the label `committeeID` to the HotStuff metrics and
// allowing for configurable name space.
type HotstuffCollector struct {
	busyDuration                   *prometheus.HistogramVec
	busyDurationCsCounter          *prometheus.CounterVec
	idleDuration                   prometheus.Histogram
	idleDurationCsCounter          prometheus.Counter
	waitDuration                   *prometheus.HistogramVec
	waitDurationCsCounter          *prometheus.CounterVec
	curView                        prometheus.Gauge
	qcView                         prometheus.Gauge
	skips                          prometheus.Counter
	timeouts                       prometheus.Counter
	timeoutDuration                prometheus.Gauge
	committeeComputationsCsCounter prometheus.Counter
	signerComputationsCsCounter    prometheus.Counter
	validatorComputationsCsCounter prometheus.Counter
	payloadProductionCsCounter     prometheus.Counter
}

func NewHotstuffCollector(chain flow.ChainID) *HotstuffCollector {

	hc := &HotstuffCollector{

		busyDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:        "busy_duration_seconds",
			Namespace:   namespaceConsensus,
			Subsystem:   subsystemHotstuff,
			Help:        "the duration of how long HotStuff's event loop has been busy processing one event",
			Buckets:     []float64{0.05, 0.2, 0.5, 1, 2, 5},
			ConstLabels: prometheus.Labels{LabelChain: chain.String()},
		}, []string{"event_type"}),
		busyDurationCsCounter: promauto.NewCounterVec(prometheus.CounterOpts{
			Name:        "busy_duration_cseconds_total",
			Namespace:   namespaceConsensus,
			Subsystem:   subsystemHotstuff,
			Help:        "total wall-clock time [unit: seconds; measured with millisecond resolution] of how long HotStuff's event loop has been busy processing events",
			ConstLabels: prometheus.Labels{LabelChain: chain.String()},
		}, []string{"event_type"}),

		idleDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:        "idle_duration_seconds",
			Namespace:   namespaceConsensus,
			Subsystem:   subsystemHotstuff,
			Help:        "the duration of how long HotStuff's event loop has been idle without processing any event",
			Buckets:     []float64{0.05, 0.2, 0.5, 1, 2, 5},
			ConstLabels: prometheus.Labels{LabelChain: chain.String()},
		}),
		idleDurationCsCounter: promauto.NewCounter(prometheus.CounterOpts{
			Name:        "idle_duration_cseconds_total",
			Namespace:   namespaceConsensus,
			Subsystem:   subsystemHotstuff,
			Help:        "total wall-clock time [unit: seconds; with float64 resolution] of how long HotStuff's event loop has been idle without processing any event",
			ConstLabels: prometheus.Labels{LabelChain: chain.String()},
		}),

		waitDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:        "wait_duration_seconds",
			Namespace:   namespaceConsensus,
			Subsystem:   subsystemHotstuff,
			Help:        "the duration of how long an event has been waited in the HotStuff event loop queue before being processed.",
			Buckets:     []float64{0.05, 0.2, 0.5, 1, 2, 5},
			ConstLabels: prometheus.Labels{LabelChain: chain.String()},
		}, []string{"event_type"}),
		waitDurationCsCounter: promauto.NewCounterVec(prometheus.CounterOpts{
			Name:        "wait_duration_cseconds_total",
			Namespace:   namespaceConsensus,
			Subsystem:   subsystemHotstuff,
			Help:        "total wall-clock time [unit: seconds; with float64 resolution] of how long an event has been waited in the HotStuff event loop queue before being processed.",
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
			Help:        "The view of the newest known qc from HotStuff",
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

		committeeComputationsCsCounter: promauto.NewCounter(prometheus.CounterOpts{
			Name:        "committee_computations_cseconds_total",
			Namespace:   namespaceConsensus,
			Subsystem:   subsystemHotstuff,
			Help:        "total wall-clock time [unit: seconds; with float64 resolution] of how long HotStuff sends computing consensus committee relations",
			ConstLabels: prometheus.Labels{LabelChain: chain.String()},
		}),

		signerComputationsCsCounter: promauto.NewCounter(prometheus.CounterOpts{
			Name:        "crypto_computations_cseconds_total",
			Namespace:   namespaceConsensus,
			Subsystem:   subsystemHotstuff,
			Help:        "total wall-clock time [unit: seconds; with float64 resolution] of how long HotStuff sends with crypto-related operations",
			ConstLabels: prometheus.Labels{LabelChain: chain.String()},
		}),

		validatorComputationsCsCounter: promauto.NewCounter(prometheus.CounterOpts{
			Name:        "message_validation_cseconds_total",
			Namespace:   namespaceConsensus,
			Subsystem:   subsystemHotstuff,
			Help:        "total wall-clock time [unit: seconds; with float64 resolution] of how long HotStuff sends with message-validation",
			ConstLabels: prometheus.Labels{LabelChain: chain.String()},
		}),

		payloadProductionCsCounter: promauto.NewCounter(prometheus.CounterOpts{
			Name:        "payload_production_cseconds_total",
			Namespace:   namespaceConsensus,
			Subsystem:   subsystemHotstuff,
			Help:        "total wall-clock time [unit: seconds; with float64 resolution] of how long HotStuff sends with payload production",
			ConstLabels: prometheus.Labels{LabelChain: chain.String()},
		}),
	}

	return hc
}

// HotStuffBusyDuration reports Metrics C6 HotStuff Busy Duration
func (hc *HotstuffCollector) HotStuffBusyDuration(duration time.Duration, event string) {
	d := duration.Seconds() // duration in seconds with resolution as much as float64
	hc.busyDuration.WithLabelValues(event).Observe(d)
	hc.busyDurationCsCounter.WithLabelValues(event).Add(d) // we count the number of wall-clock time [unit: seconds]
}

// HotStuffIdleDuration reports Metrics C6 HotStuff Idle Duration
func (hc *HotstuffCollector) HotStuffIdleDuration(duration time.Duration) {
	d := duration.Seconds() // duration in seconds with resolution as much as float64
	hc.idleDuration.Observe(d)
	hc.idleDurationCsCounter.Add(d) // we count the number of wall-clock time [unit: seconds]
}

// HotStuffWaitDuration reports Metrics C6 HotStuff Wait Duration
func (hc *HotstuffCollector) HotStuffWaitDuration(duration time.Duration, event string) {
	d := duration.Seconds() // duration in seconds with resolution as much as float64
	hc.waitDuration.WithLabelValues(event).Observe(d)
	hc.waitDurationCsCounter.WithLabelValues(event).Add(d) // we count the number of wall-clock time [unit: seconds]
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

// CommitteeProcessingDuration measures the time which the HotStuff's core logic
// spends in the hotstuff.Committee component, i.e. the time determining consensus
// committee relations.
func (hc *HotstuffCollector) CommitteeProcessingDuration(duration time.Duration) {
	// we count the number of wall-clock time [unit: seconds] with resolution as much as float64
	hc.committeeComputationsCsCounter.Add(duration.Seconds())
}

// SignerProcessingDuration reports the time which the HotStuff's core logic
// spends in the hotstuff.Signer component, i.e. the with crypto-related operations.
func (hc *HotstuffCollector) SignerProcessingDuration(duration time.Duration) {
	// we count the number of wall-clock time [unit: seconds] with resolution as much as float64
	hc.signerComputationsCsCounter.Add(duration.Seconds())
}

// ValidatorProcessingDuration reports the time which the HotStuff's core logic
// spends in the hotstuff.Validator component, i.e. the with verifying higher-level
// consensus messages.
func (hc *HotstuffCollector) ValidatorProcessingDuration(duration time.Duration) {
	// we count the number of wall-clock time [unit: seconds] with resolution as much as float64
	hc.validatorComputationsCsCounter.Add(duration.Seconds())
}

// PayloadProductionDuration reports the time which the HotStuff's core logic
// spends in the module.Builder component, i.e. the with generating block payloads
func (hc *HotstuffCollector) PayloadProductionDuration(duration time.Duration) {
	// we count the number of wall-clock time [unit: seconds] with resolution as much as float64
	hc.payloadProductionCsCounter.Add(duration.Seconds())
}
