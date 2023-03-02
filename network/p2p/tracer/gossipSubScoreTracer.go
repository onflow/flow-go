package tracer

import (
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/utils/logging"
)

const (
	PeerScoreLogMessage = "peer score snapshot update"
)

type GossipSubScoreTracer struct {
	updateInterval time.Duration // interval at which it is expecting to receive updates from the gossipsub router
	logger         zerolog.Logger

	snapshotUpdate chan struct{} // a channel to notify the snapshot update.
	snapshotLock   sync.RWMutex
	snapshot       map[peer.ID]*p2p.PeerScoreSnapshot
	idProvider     module.IdentityProvider
}

var _ p2p.PeerScoreTracer = (*GossipSubScoreTracer)(nil)

// UpdatePeerScoreSnapshots updates the tracer's snapshot of the peer scores.
func (g *GossipSubScoreTracer) UpdatePeerScoreSnapshots(snapshot map[peer.ID]*p2p.PeerScoreSnapshot) {
	g.snapshotLock.Lock()
	defer g.snapshotLock.Unlock()

	g.snapshot = snapshot
}

// UpdateInterval returns the interval at which the tracer expects to receive updates from the gossipsub router.
func (g *GossipSubScoreTracer) UpdateInterval() time.Duration {
	return g.updateInterval
}

// GetScore returns the overall score for the given peer.
func (g *GossipSubScoreTracer) GetScore(peerID peer.ID) float64 {
	g.snapshotLock.RLock()
	defer g.snapshotLock.RUnlock()

	if snapshot, ok := g.snapshot[peerID]; ok {
		return snapshot.Score
	}

	return 0
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
func (g *GossipSubScoreTracer) GetTopicScores(peerID peer.ID) (map[string]p2p.TopicScoreSnapshot, bool) {
	g.snapshotLock.RLock()
	defer g.snapshotLock.RUnlock()

	topicsSnapshot := make(map[string]p2p.TopicScoreSnapshot)
	snapshot, ok := g.snapshot[peerID]
	if !ok {
		return nil, false
	}

	// copy the topic scores into a new map
	for topic, topicSnapshot := range snapshot.Topics {
		topicsSnapshot[topic] = *topicSnapshot
	}

	return topicsSnapshot, true
}

func (g *GossipSubScoreTracer) logLoop(ctx irrecoverable.SignalerContext) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		select {
		case <-ctx.Done():
			return
		case <-g.snapshotUpdate:
			g.logPeerScores()
		}
	}
}

func (g *GossipSubScoreTracer) logPeerScores() {

}

// logPeerScore logs the peer score snapshot for the given peer.
func (g *GossipSubScoreTracer) logPeerScore(peerID peer.ID) {
	g.snapshotLock.RLock()
	defer g.snapshotLock.RUnlock()

	snapshot, ok := g.snapshot[peerID]
	if !ok {
		return
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

	lg = g.logger.With().
		Str("peer_id", peerID.String()).
		Float64("overall_score", snapshot.Score).
		Float64("app_specific_score", snapshot.AppSpecificScore).
		Float64("ip_colocation_factor", snapshot.IPColocationFactor).
		Float64("behaviour_penalty", snapshot.BehaviourPenalty).Logger()

	for topic, topicSnapshot := range snapshot.Topics {
		lg = lg.With().
			Str("topic", topic).
			Dur("time_in_mesh", topicSnapshot.TimeInMesh).
			Float64("first_message_deliveries", topicSnapshot.FirstMessageDeliveries).
			Float64("mesh_message_deliveries", topicSnapshot.MeshMessageDeliveries).
			Float64("invalid_messages", topicSnapshot.InvalidMessageDeliveries).Logger()
	}

	if snapshot.IsWarning() {
		lg.Warn().Msg(PeerScoreLogMessage)
		return
	}

	lg.Info().Msg(PeerScoreLogMessage)
}
