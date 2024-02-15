package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/module"
)

// UnicastManagerMetrics metrics collector for the unicast manager.
type UnicastManagerMetrics struct {
	// Tracks the number of times a stream creation is retried due to dial-backoff.
	createStreamRetriesDueToDialBackoff *prometheus.HistogramVec
	// Tracks the overall time it takes to create a stream, including dialing the peer and connecting to the peer due to dial-backoff.
	createStreamTimeDueToDialBackoff *prometheus.HistogramVec
	// Tracks the number of retry attempts to dial a peer during stream creation.
	dialPeerRetries *prometheus.HistogramVec
	// Tracks the time it takes to dial a peer and establish a connection during stream creation.
	dialPeerTime *prometheus.HistogramVec
	// Tracks the number of retry attempts to create the stream after peer dialing completes and a connection is established.
	createStreamOnConnRetries *prometheus.HistogramVec
	// Tracks the time it takes to create the stream after peer dialing completes and a connection is established.
	createStreamOnConnTime *prometheus.HistogramVec
	// Tracks the history of the stream retry budget updates.
	streamRetryBudgetUpdates prometheus.Histogram
	// Tracks the history of the dial retry budget updates.
	dialRetryBudgetUpdates prometheus.Histogram
	// Tracks the number of times the dial retry budget is reset to default.
	dialRetryBudgetResetToDefault prometheus.Counter
	// Tracks the number of times the stream creation retry budget is reset to default.
	streamCreationRetryBudgetResetToDefault prometheus.Counter

	prefix string
}

var _ module.UnicastManagerMetrics = (*UnicastManagerMetrics)(nil)

func NewUnicastManagerMetrics(prefix string) *UnicastManagerMetrics {
	uc := &UnicastManagerMetrics{prefix: prefix}

	uc.createStreamRetriesDueToDialBackoff = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      uc.prefix + "attempts_to_create_stream_due_to_in_progress_dial_total",
			Help:      "the number of times a stream creation is retried due to a dial in progress",
			Buckets:   []float64{1, 2, 3},
		}, []string{LabelSuccess},
	)

	uc.createStreamTimeDueToDialBackoff = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      uc.prefix + "overall_time_to_create_stream_seconds",
			Help:      "the amount of time it takes to create a stream successfully in seconds including the time to create a connection when needed",
			Buckets:   []float64{0.01, 0.1, 0.5, 1, 2, 5},
		}, []string{LabelSuccess},
	)

	uc.dialPeerRetries = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      uc.prefix + "attempts_to_dial_peer_total",
			Help:      "number of retry attempts before a connection is established successfully",
			Buckets:   []float64{1, 2, 3},
		}, []string{LabelSuccess},
	)

	uc.dialPeerTime = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      uc.prefix + "time_to_dial_peer_seconds",
			Help:      "the amount of time it takes to dial a peer and establish a connection during stream creation",
			Buckets:   []float64{0.01, 0.1, 0.5, 1, 2, 5},
		}, []string{LabelSuccess},
	)

	uc.createStreamOnConnRetries = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      uc.prefix + "attempts_to_create_stream_on_connection_total",
			Help:      "number of retry attempts before a stream is created on the available connection between two peers",
			Buckets:   []float64{1, 2, 3},
		}, []string{LabelSuccess},
	)

	uc.createStreamOnConnTime = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      uc.prefix + "time_to_create_stream_on_connection_seconds",
			Help:      "the amount of time it takes to create a stream on the available connection between two peers",
			Buckets:   []float64{0.01, 0.1, 0.5, 1, 2, 5},
		}, []string{LabelSuccess},
	)

	uc.streamRetryBudgetUpdates = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      uc.prefix + "stream_creation_retry_budget",
			Help:      "the history of the stream retry budget updates",
			Buckets:   []float64{1, 2, 3, 4, 5, 10},
		},
	)

	uc.dialRetryBudgetUpdates = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      uc.prefix + "dial_retry_budget",
			Help:      "the history of the dial retry budget updates",
			Buckets:   []float64{1, 2, 3, 4, 5, 10},
		},
	)

	uc.streamCreationRetryBudgetResetToDefault = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      uc.prefix + "stream_creation_retry_budget_reset_to_default_total",
			Help:      "the number of times the stream creation retry budget is reset to default by the unicast manager",
		})

	uc.dialRetryBudgetResetToDefault = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      uc.prefix + "dial_retry_budget_reset_to_default_total",
			Help:      "the number of times the dial retry budget is reset to default by the unicast manager",
		})

	return uc
}

// OnStreamCreated tracks the overall time taken to create a stream successfully and the number of retry attempts.
func (u *UnicastManagerMetrics) OnStreamCreated(duration time.Duration, attempts int) {
	u.createStreamRetriesDueToDialBackoff.WithLabelValues("true").Observe(float64(attempts))
	u.createStreamTimeDueToDialBackoff.WithLabelValues("true").Observe(duration.Seconds())
}

// OnStreamCreationFailure tracks the overall time taken and number of retry attempts used when the unicast manager fails to create a stream.
func (u *UnicastManagerMetrics) OnStreamCreationFailure(duration time.Duration, attempts int) {
	u.createStreamRetriesDueToDialBackoff.WithLabelValues("false").Observe(float64(attempts))
	u.createStreamTimeDueToDialBackoff.WithLabelValues("false").Observe(duration.Seconds())
}

// OnPeerDialed tracks the time it takes to dial a peer during stream creation and the number of retry attempts before a peer
// is dialed successfully.
func (u *UnicastManagerMetrics) OnPeerDialed(duration time.Duration, attempts int) {
	u.dialPeerRetries.WithLabelValues("true").Observe(float64(attempts))
	u.dialPeerTime.WithLabelValues("true").Observe(duration.Seconds())
}

// OnPeerDialFailure tracks the amount of time taken and number of retry attempts used when the unicast manager cannot dial a peer
// to establish the initial connection between the two.
func (u *UnicastManagerMetrics) OnPeerDialFailure(duration time.Duration, attempts int) {
	u.dialPeerRetries.WithLabelValues("false").Observe(float64(attempts))
	u.dialPeerTime.WithLabelValues("false").Observe(duration.Seconds())
}

// OnStreamEstablished tracks the time it takes to create a stream successfully on the available open connection during stream
// creation and the number of retry attempts.
func (u *UnicastManagerMetrics) OnStreamEstablished(duration time.Duration, attempts int) {
	u.createStreamOnConnRetries.WithLabelValues("true").Observe(float64(attempts))
	u.createStreamOnConnTime.WithLabelValues("true").Observe(duration.Seconds())
}

// OnEstablishStreamFailure tracks the amount of time taken and number of retry attempts used when the unicast manager cannot establish
// a stream on the open connection between two peers.
func (u *UnicastManagerMetrics) OnEstablishStreamFailure(duration time.Duration, attempts int) {
	u.createStreamOnConnRetries.WithLabelValues("false").Observe(float64(attempts))
	u.createStreamOnConnTime.WithLabelValues("false").Observe(duration.Seconds())
}

// OnStreamCreationRetryBudgetUpdated tracks the history of the stream creation retry budget updates.
func (u *UnicastManagerMetrics) OnStreamCreationRetryBudgetUpdated(budget uint64) {
	u.dialRetryBudgetUpdates.Observe(float64(budget))
}

// OnDialRetryBudgetUpdated tracks the history of the dial retry budget updates.
func (u *UnicastManagerMetrics) OnDialRetryBudgetUpdated(budget uint64) {
	u.streamRetryBudgetUpdates.Observe(float64(budget))
}

// OnDialRetryBudgetResetToDefault tracks the number of times the dial retry budget is reset to default.
func (u *UnicastManagerMetrics) OnDialRetryBudgetResetToDefault() {
	u.dialRetryBudgetResetToDefault.Inc()
}

// OnStreamCreationRetryBudgetResetToDefault tracks the number of times the stream creation retry budget is reset to default.
func (u *UnicastManagerMetrics) OnStreamCreationRetryBudgetResetToDefault() {
	u.streamCreationRetryBudgetResetToDefault.Inc()
}
