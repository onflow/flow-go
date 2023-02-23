package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/module"
)

// UnicastManagerMetrics metrics collector for the unicast manager.
type UnicastManagerMetrics struct {
	// createStreamAttempts tracks the number of retry attempts to create a stream.
	createStreamAttempts *prometheus.HistogramVec
	// createStreamDuration tracks the overall time it takes to create a stream, this time includes
	// time spent dialing the peer and time spent connecting to the peer and creating the stream.
	createStreamDuration *prometheus.HistogramVec
	// dialPeerAttempts tracks the number of retry attempts to dial a peer during stream creation.
	dialPeerAttempts *prometheus.HistogramVec
	// dialPeerDuration tracks the time it takes to dial a peer and establish a connection.
	dialPeerDuration *prometheus.HistogramVec
	// createStreamToPeerAttempts tracks the number of retry attempts to create the stream after peer dialing completes and a connection is established.
	createStreamToPeerAttempts *prometheus.HistogramVec
	// createStreamToPeerDuration tracks the time it takes to create the stream after peer dialing completes and a connection is established.
	createStreamToPeerDuration *prometheus.HistogramVec

	prefix string
}

var _ module.UnicastManagerMetrics = (*UnicastManagerMetrics)(nil)

func NewUnicastManagerMetrics(prefix string) *UnicastManagerMetrics {
	uc := &UnicastManagerMetrics{prefix: prefix}

	uc.createStreamAttempts = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      uc.prefix + "create_stream_attempts",
			Help:      "number of retry attempts before stream created successfully",
			Buckets:   []float64{1, 2, 3},
		}, []string{LabelResult},
	)

	uc.createStreamDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      uc.prefix + "create_stream_duration",
			Help:      "the amount of time it takes to create a stream successfully",
			Buckets:   []float64{0.01, 0.1, 0.5, 1, 2, 5},
		}, []string{LabelResult},
	)

	uc.dialPeerAttempts = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      uc.prefix + "dial_peer_attempts",
			Help:      "number of retry attempts before a peer is dialed successfully",
			Buckets:   []float64{1, 2, 3},
		}, []string{LabelResult},
	)

	uc.dialPeerDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      uc.prefix + "dial_peer_duration",
			Help:      "the amount of time it takes to dial a peer during stream creation",
			Buckets:   []float64{0.01, 0.1, 0.5, 1, 2, 5},
		}, []string{LabelResult},
	)

	uc.createStreamToPeerAttempts = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      uc.prefix + "create_stream_to_peer_attempts",
			Help:      "number of retry attempts before a stream is created on the available connection between two peers",
			Buckets:   []float64{1, 2, 3},
		}, []string{LabelResult},
	)

	uc.createStreamToPeerDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      uc.prefix + "create_stream_to_peer_duration",
			Help:      "the amount of time it takes to create a stream on the available connection between two peers",
			Buckets:   []float64{0.01, 0.1, 0.5, 1, 2, 5},
		}, []string{LabelResult},
	)

	return uc
}

// OnCreateStream tracks the overall time it takes to create a stream successfully and the number of retry attempts.
func (u *UnicastManagerMetrics) OnCreateStream(duration time.Duration, attempts int, result string) {
	u.createStreamAttempts.WithLabelValues(result).Observe(float64(attempts))
	u.createStreamDuration.WithLabelValues(result).Observe(duration.Seconds())
}

// OnDialPeer tracks the time it takes to dial a peer during stream creation and the number of retry attempts.
func (u *UnicastManagerMetrics) OnDialPeer(duration time.Duration, attempts int, result string) {
	u.dialPeerAttempts.WithLabelValues(result).Observe(float64(attempts))
	u.dialPeerDuration.WithLabelValues(result).Observe(duration.Seconds())
}

// OnCreateStreamToPeer tracks the time it takes to create a stream on the available open connection during stream
// creation and the number of retry attempts.
func (u *UnicastManagerMetrics) OnCreateStreamToPeer(duration time.Duration, attempts int, result string) {
	u.createStreamToPeerAttempts.WithLabelValues(result).Observe(float64(attempts))
	u.createStreamToPeerDuration.WithLabelValues(result).Observe(duration.Seconds())
}
