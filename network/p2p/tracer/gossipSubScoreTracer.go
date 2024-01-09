package tracer

import (
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	p2plogging "github.com/onflow/flow-go/network/p2p/logging"
	"github.com/onflow/flow-go/utils/logging"
)

const (
	PeerScoreLogMessage = "peer score snapshot update"
)

// GossipSubScoreTracer is a tracer that keeps track of the peer scores of the gossipsub router.
// It is used to log the peer scores at regular intervals.
type GossipSubScoreTracer struct {
	component.Component

	updateInterval time.Duration // interval at which it is expecting to receive updates from the gossipsub router
	logger         zerolog.Logger
	collector      module.GossipSubScoringMetrics

	snapshotUpdate    chan struct{} // a channel to notify the snapshot update.
	snapshotLock      sync.RWMutex
	snapshot          map[peer.ID]*p2p.PeerScoreSnapshot
	snapShotUpdateReq chan map[peer.ID]*p2p.PeerScoreSnapshot
	idProvider        module.IdentityProvider
}

var _ p2p.PeerScoreTracer = (*GossipSubScoreTracer)(nil)

func NewGossipSubScoreTracer(
	logger zerolog.Logger,
	provider module.IdentityProvider,
	collector module.GossipSubScoringMetrics,
	updateInterval time.Duration) *GossipSubScoreTracer {
	g := &GossipSubScoreTracer{
		logger:            logger.With().Str("component", "gossipsub_score_tracer").Logger(),
		updateInterval:    updateInterval,
		collector:         collector,
		snapshotUpdate:    make(chan struct{}, 1),
		snapShotUpdateReq: make(chan map[peer.ID]*p2p.PeerScoreSnapshot, 1),
		snapshot:          make(map[peer.ID]*p2p.PeerScoreSnapshot),
		idProvider:        provider,
	}

	g.Component = component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()
			g.logLoop(ctx)
		}).
		Build()

	return g
}

// UpdatePeerScoreSnapshots updates the tracer's snapshot of the peer scores. It is called by the gossipsub router.
// It is non-blocking and asynchrounous. If there is no update pending, it queues an update. If there is an update pending,
// it drops the update.
func (g *GossipSubScoreTracer) UpdatePeerScoreSnapshots(snapshot map[peer.ID]*p2p.PeerScoreSnapshot) {
	select {
	case g.snapShotUpdateReq <- snapshot:
	default:
		// if the channel is full, we drop the update. This should rarely happen as the log loop should be able to keep up.
		// if it does happen, it means that the log loop is not running or is blocked. In this case, we don't want to block
		// the main thread.
		g.logger.Warn().Msg("dropping peer score snapshot update, channel full")
	}
}

// UpdateInterval returns the interval at which the tracer expects to receive updates from the gossipsub router.
func (g *GossipSubScoreTracer) UpdateInterval() time.Duration {
	return g.updateInterval
}

// GetScore returns the overall score for the given peer.
func (g *GossipSubScoreTracer) GetScore(peerID peer.ID) (float64, bool) {
	g.snapshotLock.RLock()
	defer g.snapshotLock.RUnlock()

	if snapshot, ok := g.snapshot[peerID]; ok {
		return snapshot.Score, true
	}

	return 0, false
}

// GetAppScore returns the application score for the given peer.
func (g *GossipSubScoreTracer) GetAppScore(peerID peer.ID) (float64, bool) {
	g.snapshotLock.RLock()
	defer g.snapshotLock.RUnlock()

	if snapshot, ok := g.snapshot[peerID]; ok {
		return snapshot.AppSpecificScore, true
	}

	return 0, false
}

// GetIPColocationFactor returns the IP colocation factor for the given peer.
func (g *GossipSubScoreTracer) GetIPColocationFactor(peerID peer.ID) (float64, bool) {
	g.snapshotLock.RLock()
	defer g.snapshotLock.RUnlock()

	if snapshot, ok := g.snapshot[peerID]; ok {
		return snapshot.IPColocationFactor, true
	}

	return 0, false
}

// GetBehaviourPenalty returns the behaviour penalty for the given peer.
func (g *GossipSubScoreTracer) GetBehaviourPenalty(peerID peer.ID) (float64, bool) {
	g.snapshotLock.RLock()
	defer g.snapshotLock.RUnlock()

	if snapshot, ok := g.snapshot[peerID]; ok {
		return snapshot.BehaviourPenalty, true
	}

	return 0, false
}

// GetTopicScores returns the topic scores for the given peer.
// The returned map is keyed by topic name.
func (g *GossipSubScoreTracer) GetTopicScores(peerID peer.ID) (map[string]p2p.TopicScoreSnapshot, bool) {
	g.snapshotLock.RLock()
	defer g.snapshotLock.RUnlock()

	snapshot, ok := g.snapshot[peerID]
	if !ok {
		return nil, false
	}

	topicsSnapshot := make(map[string]p2p.TopicScoreSnapshot)
	// copy the topic scores into a new map
	for topic, topicSnapshot := range snapshot.Topics {
		topicsSnapshot[topic] = *topicSnapshot
	}

	return topicsSnapshot, true
}

func (g *GossipSubScoreTracer) logLoop(ctx irrecoverable.SignalerContext) {
	g.logger.Debug().Msg("starting log loop")
	for {
		select {
		case <-ctx.Done():
			g.logger.Debug().Msg("stopping log loop")
			return
		default:
		}

		select {
		case <-ctx.Done():
			g.logger.Debug().Msg("stopping log loop")
			return
		case snapshot := <-g.snapShotUpdateReq:
			g.logger.Debug().Msg("received snapshot update")
			g.updateSnapshot(snapshot)
			g.logger.Debug().Msg("snapshot updated")
			g.logPeerScores()
			g.logger.Debug().Msg("peer scores logged")
		}
	}
}

// updateSnapshot updates the tracer's snapshot of the peer scores.
// It is called by the log loop, it is a blocking and synchronous call.
func (g *GossipSubScoreTracer) updateSnapshot(snapshot map[peer.ID]*p2p.PeerScoreSnapshot) {
	g.snapshotLock.Lock()
	defer g.snapshotLock.Unlock()

	g.snapshot = snapshot
}

// logPeerScores logs the peer score snapshots for all peers.
func (g *GossipSubScoreTracer) logPeerScores() {
	g.snapshotLock.RLock()
	defer g.snapshotLock.RUnlock()

	g.logger.Debug().Msg("logging peer scores")
	warningStateCount := uint(0)

	for peerID := range g.snapshot {
		warning := g.logPeerScore(peerID)
		if warning {
			warningStateCount++
		}
	}

	g.collector.SetWarningStateCount(warningStateCount)
	g.logger.Debug().Msg("finished logging peer scores")
}

// logPeerScore logs the peer score snapshot for the given peer.
// It also updates the score-related metrics.
// The return value indicates whether the peer score is in the warning state.
// Note: this function is not thread-safe and should be called with the lock held.
func (g *GossipSubScoreTracer) logPeerScore(peerID peer.ID) bool {
	snapshot, ok := g.snapshot[peerID]
	if !ok {
		return false
	}

	var lg zerolog.Logger

	identity, valid := g.idProvider.ByPeerID(peerID)
	if !valid {
		lg = g.logger.With().
			Str("flow_id", "unknown").
			Str("role", "unknown").Logger()
	} else {
		lg = g.logger.With().
			Hex("flow_id", logging.ID(identity.NodeID)).
			Str("role", identity.Role.String()).Logger()
	}

	lg = lg.With().
		Str("peer_id", p2plogging.PeerId(peerID)).
		Float64("overall_score", snapshot.Score).
		Float64("app_specific_score", snapshot.AppSpecificScore).
		Float64("ip_colocation_factor", snapshot.IPColocationFactor).
		Float64("behaviour_penalty", snapshot.BehaviourPenalty).Logger()

	g.collector.OnOverallPeerScoreUpdated(snapshot.Score)
	g.collector.OnAppSpecificScoreUpdated(snapshot.AppSpecificScore)
	g.collector.OnIPColocationFactorUpdated(snapshot.IPColocationFactor)
	g.collector.OnBehaviourPenaltyUpdated(snapshot.BehaviourPenalty)

	for topic, topicSnapshot := range snapshot.Topics {
		lg = lg.With().
			Str("topic", topic).
			Dur("time_in_mesh", topicSnapshot.TimeInMesh).
			Float64("first_message_deliveries", topicSnapshot.FirstMessageDeliveries).
			Float64("mesh_message_deliveries", topicSnapshot.MeshMessageDeliveries).
			Float64("invalid_messages", topicSnapshot.InvalidMessageDeliveries).Logger()

		g.collector.OnFirstMessageDeliveredUpdated(channels.Topic(topic), topicSnapshot.FirstMessageDeliveries)
		g.collector.OnMeshMessageDeliveredUpdated(channels.Topic(topic), topicSnapshot.MeshMessageDeliveries)
		g.collector.OnInvalidMessageDeliveredUpdated(channels.Topic(topic), topicSnapshot.InvalidMessageDeliveries)
		g.collector.OnTimeInMeshUpdated(channels.Topic(topic), topicSnapshot.TimeInMesh)
	}

	if snapshot.IsWarning() {
		lg.Warn().Msg(PeerScoreLogMessage)
		return true
	}

	lg.Debug().Msg(PeerScoreLogMessage)
	return false
}
