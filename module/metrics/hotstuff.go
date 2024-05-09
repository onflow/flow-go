package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// HotStuff Metrics
const (
	HotstuffEventTypeLocalTimeout = "localtimeout"
	HotstuffEventTypeOnProposal   = "onproposal"
	HotstuffEventTypeOnQC         = "onqc"
	HotstuffEventTypeOnTC         = "ontc"
	HotstuffEventTypeOnPartialTc  = "onpartialtc"
)

// HotstuffCollector implements only the metrics emitted by the HotStuff core logic.
// We have multiple instances of HotStuff running within Flow: Consensus Nodes form
// the main consensus committee. In addition each Collector node cluster runs their
// own HotStuff instance. Depending on the node role, the name space is different. Furthermore,
// even within the `collection` name space, we need to separate metrics between the different
// clusters. We do this by adding the label `committeeID` to the HotStuff metrics and
// allowing for configurable name space.
type HotstuffCollector struct {
	busyDuration                  *prometheus.HistogramVec
	idleDuration                  prometheus.Histogram
	waitDuration                  *prometheus.HistogramVec
	curView                       prometheus.Gauge
	qcView                        prometheus.Gauge
	tcView                        prometheus.Gauge
	skips                         prometheus.Counter
	timeouts                      prometheus.Counter
	timeoutDuration               prometheus.Gauge
	voteProcessingDuration        prometheus.Histogram
	timeoutProcessingDuration     prometheus.Histogram
	blockProcessingDuration       prometheus.Histogram
	committeeComputationsDuration prometheus.Histogram
	signerComputationsDuration    prometheus.Histogram
	validatorComputationsDuration prometheus.Histogram
	payloadProductionDuration     prometheus.Histogram
	timeoutCollectorsRange        *prometheus.GaugeVec
	numberOfActiveCollectors      prometheus.Gauge
}

var _ module.HotstuffMetrics = (*HotstuffCollector)(nil)

func NewHotstuffCollector(chain flow.ChainID) *HotstuffCollector {

	hc := &HotstuffCollector{

		busyDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:        "busy_duration_seconds",
			Namespace:   namespaceConsensus,
			Subsystem:   subsystemHotstuff,
			Help:        "duration [seconds; measured with float64 precision] of how long HotStuff's event loop has been busy processing one event",
			Buckets:     []float64{0.05, 0.2, 0.5, 1, 2, 5},
			ConstLabels: prometheus.Labels{LabelChain: chain.String()},
		}, []string{"event_type"}),

		idleDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:        "idle_duration_seconds",
			Namespace:   namespaceConsensus,
			Subsystem:   subsystemHotstuff,
			Help:        "duration [seconds; measured with float64 precision] of how long HotStuff's event loop has been idle without processing any event",
			Buckets:     []float64{0.05, 0.2, 0.5, 1, 2, 5},
			ConstLabels: prometheus.Labels{LabelChain: chain.String()},
		}),

		waitDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:        "wait_duration_seconds",
			Namespace:   namespaceConsensus,
			Subsystem:   subsystemHotstuff,
			Help:        "duration [seconds; measured with float64 precision] of how long an event has been waited in the HotStuff event loop queue before being processed.",
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
			Help:        "The view of the newest known QC from HotStuff",
			ConstLabels: prometheus.Labels{LabelChain: chain.String()},
		}),

		tcView: promauto.NewGauge(prometheus.GaugeOpts{
			Name:        "tc_view",
			Namespace:   namespaceConsensus,
			Subsystem:   subsystemHotstuff,
			Help:        "The view of the newest known TC from HotStuff",
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
			Help:        "The number of views that this replica left due to observing a TC",
			ConstLabels: prometheus.Labels{LabelChain: chain.String()},
		}),

		timeoutDuration: promauto.NewGauge(prometheus.GaugeOpts{
			Name:        "timeout_seconds",
			Namespace:   namespaceConsensus,
			Subsystem:   subsystemHotstuff,
			Help:        "The current length of the timeout",
			ConstLabels: prometheus.Labels{LabelChain: chain.String()},
		}),

		committeeComputationsDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:        "committee_computations_seconds",
			Namespace:   namespaceConsensus,
			Subsystem:   subsystemHotstuff,
			Help:        "duration [seconds; measured with float64 precision] of how long HotStuff sends computing consensus committee relations",
			Buckets:     []float64{0.02, 0.05, 0.1, 0.2, 0.5, 1, 2},
			ConstLabels: prometheus.Labels{LabelChain: chain.String()},
		}),

		signerComputationsDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:        "crypto_computations_seconds",
			Namespace:   namespaceConsensus,
			Subsystem:   subsystemHotstuff,
			Help:        "duration [seconds; measured with float64 precision] of how long HotStuff sends with crypto-related operations",
			Buckets:     []float64{0.02, 0.05, 0.1, 0.2, 0.5, 1, 2},
			ConstLabels: prometheus.Labels{LabelChain: chain.String()},
		}),

		validatorComputationsDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:        "message_validation_seconds",
			Namespace:   namespaceConsensus,
			Subsystem:   subsystemHotstuff,
			Help:        "duration [seconds; measured with float64 precision] of how long HotStuff sends with message-validation",
			Buckets:     []float64{0.02, 0.05, 0.1, 0.2, 0.5, 1, 2},
			ConstLabels: prometheus.Labels{LabelChain: chain.String()},
		}),

		payloadProductionDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:        "payload_production_seconds",
			Namespace:   namespaceConsensus,
			Subsystem:   subsystemHotstuff,
			Help:        "duration [seconds; measured with float64 precision] of how long HotStuff sends with payload production",
			Buckets:     []float64{0.02, 0.05, 0.1, 0.2, 0.5, 1, 2},
			ConstLabels: prometheus.Labels{LabelChain: chain.String()},
		}),
		blockProcessingDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:        "block_processing_seconds",
			Namespace:   namespaceConsensus,
			Subsystem:   subsystemHotstuff,
			Help:        "duration [seconds; measured with float64 precision] of how long compliance engine processes one block",
			Buckets:     []float64{0.02, 0.05, 0.1, 0.2, 0.5, 1, 2},
			ConstLabels: prometheus.Labels{LabelChain: chain.String()},
		}),
		voteProcessingDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:        "vote_processing_seconds",
			Namespace:   namespaceConsensus,
			Subsystem:   subsystemHotstuff,
			Help:        "duration [seconds; measured with float64 precision] of how long VoteAggregator processes one message",
			Buckets:     []float64{0.02, 0.05, 0.1, 0.2, 0.5, 1, 2},
			ConstLabels: prometheus.Labels{LabelChain: chain.String()},
		}),
		timeoutProcessingDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:        "timeout_object_processing_seconds",
			Namespace:   namespaceConsensus,
			Subsystem:   subsystemHotstuff,
			Help:        "duration [seconds; measured with float64 precision] of how long TimeoutAggregator processes one message",
			Buckets:     []float64{0.02, 0.05, 0.1, 0.2, 0.5, 1, 2},
			ConstLabels: prometheus.Labels{LabelChain: chain.String()},
		}),
		timeoutCollectorsRange: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name:        "timeout_collectors_range",
			Namespace:   namespaceConsensus,
			Subsystem:   subsystemHotstuff,
			Help:        "lowest and highest views that we are maintaining TimeoutCollectors for",
			ConstLabels: prometheus.Labels{LabelChain: chain.String()},
		}, []string{"prefix"}),
		numberOfActiveCollectors: promauto.NewGauge(prometheus.GaugeOpts{
			Name:        "active_collectors",
			Namespace:   namespaceConsensus,
			Subsystem:   subsystemHotstuff,
			Help:        "number of active TimeoutCollectors that the TimeoutAggregator component currently maintains",
			ConstLabels: prometheus.Labels{LabelChain: chain.String()},
		}),
	}

	return hc
}

// HotStuffBusyDuration reports Metrics C6 HotStuff Busy Duration
func (hc *HotstuffCollector) HotStuffBusyDuration(duration time.Duration, event string) {
	hc.busyDuration.WithLabelValues(event).Observe(duration.Seconds()) // unit: seconds; with float64 precision
}

// HotStuffIdleDuration reports Metrics C6 HotStuff Idle Duration
func (hc *HotstuffCollector) HotStuffIdleDuration(duration time.Duration) {
	hc.idleDuration.Observe(duration.Seconds()) // unit: seconds; with float64 precision
}

// HotStuffWaitDuration reports Metrics C6 HotStuff Idle Duration - the time between receiving and
// enqueueing a message to beginning to process that message.
func (hc *HotstuffCollector) HotStuffWaitDuration(duration time.Duration, event string) {
	hc.waitDuration.WithLabelValues(event).Observe(duration.Seconds()) // unit: seconds; with float64 precision
}

// CountSkipped counts the number of skips we did.
func (hc *HotstuffCollector) CountSkipped() {
	hc.skips.Inc()
}

// CountTimeout tracks the number of views that this replica left due to observing a TC.
func (hc *HotstuffCollector) CountTimeout() {
	hc.timeouts.Inc()
}

// SetCurView reports Metrics C8: Current View
func (hc *HotstuffCollector) SetCurView(view uint64) {
	hc.curView.Set(float64(view))
}

// SetQCView reports Metrics C9: View of Newest Known QC
func (hc *HotstuffCollector) SetQCView(view uint64) {
	hc.qcView.Set(float64(view))
}

// SetTCView reports the view of the newest known TC
func (hc *HotstuffCollector) SetTCView(view uint64) {
	hc.tcView.Set(float64(view))
}

// BlockProcessingDuration measures the time which the compliance engine
// spends to process one block proposal.
func (hc *HotstuffCollector) BlockProcessingDuration(duration time.Duration) {
	hc.blockProcessingDuration.Observe(duration.Seconds())
}

// VoteProcessingDuration reports the processing time for a single vote
func (hc *HotstuffCollector) VoteProcessingDuration(duration time.Duration) {
	hc.voteProcessingDuration.Observe(duration.Seconds())
}

// TimeoutObjectProcessingDuration reports the processing time for a TimeoutObject
func (hc *HotstuffCollector) TimeoutObjectProcessingDuration(duration time.Duration) {
	hc.timeoutProcessingDuration.Observe(duration.Seconds())
}

// SetTimeout sets the current timeout duration.
func (hc *HotstuffCollector) SetTimeout(duration time.Duration) {
	hc.timeoutDuration.Set(duration.Seconds()) // unit: seconds; with float64 precision
}

// CommitteeProcessingDuration measures the time which the HotStuff's core logic
// spends in the hotstuff.Committee component, i.e. the time determining consensus
// committee relations.
func (hc *HotstuffCollector) CommitteeProcessingDuration(duration time.Duration) {
	hc.committeeComputationsDuration.Observe(duration.Seconds()) // unit: seconds; with float64 precision
}

// SignerProcessingDuration reports the time which the HotStuff's core logic
// spends in the hotstuff.Signer component, i.e. the with crypto-related operations.
func (hc *HotstuffCollector) SignerProcessingDuration(duration time.Duration) {
	hc.signerComputationsDuration.Observe(duration.Seconds()) // unit: seconds; with float64 precision
}

// ValidatorProcessingDuration reports the time which the HotStuff's core logic
// spends in the hotstuff.Validator component, i.e. the with verifying higher-level
// consensus messages.
func (hc *HotstuffCollector) ValidatorProcessingDuration(duration time.Duration) {
	hc.validatorComputationsDuration.Observe(duration.Seconds()) // unit: seconds; with float64 precision
}

// PayloadProductionDuration reports the time which the HotStuff's core logic
// spends in the module.Builder component, i.e. the with generating block payloads
func (hc *HotstuffCollector) PayloadProductionDuration(duration time.Duration) {
	hc.payloadProductionDuration.Observe(duration.Seconds()) // unit: seconds; with float64 precision
}

// TimeoutCollectorsRange collects information from the node's `TimeoutAggregator` component.
// Specifically, it measurers the number of views for which we are currently collecting timeouts
// (i.e. the number of `TimeoutCollector` instances we are maintaining) and their lowest/highest view.
func (hc *HotstuffCollector) TimeoutCollectorsRange(lowestRetainedView uint64, newestViewCreatedCollector uint64, activeCollectors int) {
	hc.timeoutCollectorsRange.WithLabelValues("lowest_view_of_active_timeout_collectors").Set(float64(lowestRetainedView))
	hc.timeoutCollectorsRange.WithLabelValues("newest_view_of_active_timeout_collectors").Set(float64(newestViewCreatedCollector))
	hc.numberOfActiveCollectors.Set(float64(activeCollectors))
}
