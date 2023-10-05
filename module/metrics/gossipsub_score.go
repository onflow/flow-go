package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network/channels"
)

type GossipSubScoreMetrics struct {
	peerScore           prometheus.Histogram
	appSpecificScore    prometheus.Histogram
	behaviourPenalty    prometheus.Histogram
	ipCollocationFactor prometheus.Histogram

	timeInMesh             prometheus.HistogramVec
	meshMessageDelivery    prometheus.HistogramVec
	firstMessageDelivery   prometheus.HistogramVec
	invalidMessageDelivery prometheus.HistogramVec

	// warningStateGauge is a gauge that keeps track of the number of peers in the warning state.
	warningStateGauge prometheus.Gauge
}

var _ module.GossipSubScoringMetrics = (*GossipSubScoreMetrics)(nil)

func NewGossipSubScoreMetrics(prefix string) *GossipSubScoreMetrics {
	gs := &GossipSubScoreMetrics{}

	gs.peerScore = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_overall_peer_score",
			Help:      "overall peer score from gossipsub peer scoring",
			Buckets:   []float64{-100, 0, 100},
		},
	)

	gs.appSpecificScore = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_app_specific_score",
			Help:      "app specific score from gossipsub peer scoring",
			Buckets:   []float64{-100, 0, 100},
		},
	)

	gs.behaviourPenalty = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_behaviour_penalty_score",
			Help:      "behaviour penalty from gossipsub peer scoring",
			Buckets:   []float64{10, 100, 1000},
		},
	)

	gs.ipCollocationFactor = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_ip_collocation_factor_score",
			Help:      "ip collocation factor from gossipsub peer scoring",
			Buckets:   []float64{10, 100, 1000},
		},
	)

	gs.timeInMesh = *promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_time_in_mesh_quantum_count",
			Help:      "time in mesh from gossipsub scoring; number of the time quantum a peer has been in the mesh",
			Buckets:   []float64{1, 24, 168}, // 1h, 1d, 1w with quantum of 1h
		},
		[]string{LabelChannel},
	)

	gs.meshMessageDelivery = *promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_mesh_message_delivery",
			Buckets:   []float64{100, 1000, 10_000},
			Help:      "mesh message delivery from gossipsub peer scoring; number of messages delivered to the mesh of local peer; decayed over time; and capped at certain value",
		},
		[]string{LabelChannel},
	)

	gs.invalidMessageDelivery = *promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Buckets:   []float64{10, 100, 1000},
			Name:      prefix + "gossipsub_invalid_message_delivery_count",
			Help:      "invalid message delivery from gossipsub peer scoring; number of invalid messages delivered to the mesh of local peer; decayed over time; and capped at certain value",
		},
		[]string{LabelChannel},
	)

	gs.firstMessageDelivery = *promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Buckets:   []float64{100, 1000, 10_000},
			Name:      prefix + "gossipsub_first_message_delivery_count",
			Help:      "first message delivery from gossipsub peer scoring; number of fresh messages delivered to the mesh of local peer; decayed over time; and capped at certain value",
		},
		[]string{LabelChannel},
	)

	gs.warningStateGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_warning_state_peers_total",
			Help:      "number of peers in the warning state",
		},
	)

	return gs
}

func (g *GossipSubScoreMetrics) OnOverallPeerScoreUpdated(score float64) {
	g.peerScore.Observe(score)
}

func (g *GossipSubScoreMetrics) OnAppSpecificScoreUpdated(score float64) {
	g.appSpecificScore.Observe(score)
}

func (g *GossipSubScoreMetrics) OnIPColocationFactorUpdated(factor float64) {
	g.ipCollocationFactor.Observe(factor)
}

func (g *GossipSubScoreMetrics) OnBehaviourPenaltyUpdated(penalty float64) {
	g.behaviourPenalty.Observe(penalty)
}

func (g *GossipSubScoreMetrics) OnTimeInMeshUpdated(topic channels.Topic, duration time.Duration) {
	g.timeInMesh.WithLabelValues(string(topic)).Observe(duration.Seconds())
}

func (g *GossipSubScoreMetrics) OnFirstMessageDeliveredUpdated(topic channels.Topic, f float64) {
	g.firstMessageDelivery.WithLabelValues(string(topic)).Observe(f)
}

func (g *GossipSubScoreMetrics) OnMeshMessageDeliveredUpdated(topic channels.Topic, f float64) {
	g.meshMessageDelivery.WithLabelValues(string(topic)).Observe(f)
}

func (g *GossipSubScoreMetrics) OnInvalidMessageDeliveredUpdated(topic channels.Topic, f float64) {
	g.invalidMessageDelivery.WithLabelValues(string(topic)).Observe(f)
}

func (g *GossipSubScoreMetrics) SetWarningStateCount(u uint) {
	g.warningStateGauge.Set(float64(u))
}
