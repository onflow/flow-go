package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network/channels"
)

var (
	// gossipSubScoreBuckets is a list of buckets for gossipsub score metrics.
	// There is almost no limit to the score, so we use a large range of buckets to capture the full range.
	gossipSubScoreBuckets = []float64{-10e6, -10e5, -10e4, -10e3, -10e2, -10e1, -10e0, 0, 10e0, 10e1, 10e2, 10e3, 10e4, 10e5, 10e6}
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
			Buckets:   gossipSubScoreBuckets,
		},
	)

	gs.appSpecificScore = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_app_specific_score",
			Help:      "app specific score from gossipsub peer scoring",
			Buckets:   gossipSubScoreBuckets,
		},
	)

	gs.behaviourPenalty = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_behaviour_penalty_score",
			Help:      "behaviour penalty from gossipsub peer scoring",
			Buckets:   gossipSubScoreBuckets,
		},
	)

	gs.ipCollocationFactor = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_ip_collocation_factor_score",
			Help:      "ip collocation factor from gossipsub peer scoring",
			Buckets:   gossipSubScoreBuckets,
		},
	)

	gs.timeInMesh = *promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_time_in_mesh_score",
			Help:      "time in mesh from gossipsub scoring",
			Buckets:   gossipSubScoreBuckets,
		},
		[]string{LabelChannel},
	)

	gs.meshMessageDelivery = *promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_mesh_message_delivery_score",
			Help:      "mesh message delivery from gossipsub peer scoring",
		},
		[]string{LabelChannel},
	)

	gs.invalidMessageDelivery = *promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_invalid_message_delivery_score",
			Help:      "invalid message delivery from gossipsub peer scoring",
		},
		[]string{LabelChannel},
	)

	gs.firstMessageDelivery = *promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespaceNetwork,
			Subsystem: subsystemGossip,
			Name:      prefix + "gossipsub_first_message_delivery_score",
			Help:      "first message delivery from gossipsub peer scoring",
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
