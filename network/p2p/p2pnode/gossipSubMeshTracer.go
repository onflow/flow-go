package p2pnode

import (
	"sync"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/utils/logging"
)

// GossipSubMeshTracer is a tracer that tracks the local mesh peers for each topic.
// It also logs the mesh peers and updates the local mesh size metric.
type GossipSubMeshTracer struct {
	component.Component

	topicMeshMu    sync.RWMutex                    // to protect topicMeshMap
	topicMeshMap   map[string]map[peer.ID]struct{} // map of local mesh peers by topic.
	logger         zerolog.Logger
	idProvider     module.IdentityProvider
	loggerInterval time.Duration
	metrics        module.GossipSubLocalMeshMetrics
}

var _ p2p.PubSubTracer = (*GossipSubMeshTracer)(nil)

func NewGossipSubMeshTracer(
	logger zerolog.Logger,
	metrics module.GossipSubLocalMeshMetrics,
	idProvider module.IdentityProvider,
	loggerInterval time.Duration) *GossipSubMeshTracer {

	g := &GossipSubMeshTracer{
		topicMeshMap:   make(map[string]map[peer.ID]struct{}),
		idProvider:     idProvider,
		metrics:        metrics,
		logger:         logger.With().Str("component", "gossip_sub_topology_tracer").Logger(),
		loggerInterval: loggerInterval,
	}

	g.Component = component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			go g.logLoop(ctx, ready)
		}).
		Build()

	return g
}

// GetMeshPeers returns the local mesh peers for the given topic.
func (t *GossipSubMeshTracer) GetMeshPeers(topic string) []peer.ID {
	t.topicMeshMu.RLock()
	defer t.topicMeshMu.RUnlock()

	peers := make([]peer.ID, 0, len(t.topicMeshMap[topic]))
	for p := range t.topicMeshMap[topic] {
		peers = append(peers, p)
	}
	return peers
}

// Graft is called when a peer is added to a topic mesh. The tracer uses this to track the mesh peers.
func (t *GossipSubMeshTracer) Graft(p peer.ID, topic string) {
	t.topicMeshMu.Lock()
	defer t.topicMeshMu.Unlock()

	lg := t.logger.With().Str("topic", topic).Str("peer_id", p.String()).Logger()

	if _, ok := t.topicMeshMap[topic]; !ok {
		t.topicMeshMap[topic] = make(map[peer.ID]struct{})
	}
	t.topicMeshMap[topic][p] = struct{}{}

	id, exists := t.idProvider.ByPeerID(p)
	if !exists {
		lg.Warn().
			Bool(logging.KeySuspicious, true).
			Msg("grafted peer not found in identity provider")
		return
	}

	lg.Info().Hex("flow_id", logging.ID(id.NodeID)).Str("role", id.Role.String()).Msg("grafted peer")
	t.metrics.OnLocalMeshSizeUpdated(topic, len(t.topicMeshMap[topic]))
}

// Prune is called when a peer is removed from a topic mesh. The tracer uses this to track the mesh peers.
func (t *GossipSubMeshTracer) Prune(p peer.ID, topic string) {
	t.topicMeshMu.Lock()
	defer t.topicMeshMu.Unlock()

	lg := t.logger.With().Str("topic", topic).Str("peer_id", p.String()).Logger()

	if _, ok := t.topicMeshMap[topic]; !ok {
		return
	}
	delete(t.topicMeshMap[topic], p)

	id, exists := t.idProvider.ByPeerID(p)
	if !exists {
		lg.Warn().
			Bool(logging.KeySuspicious, true).
			Msg("pruned peer not found in identity provider")
		return
	}

	lg.Info().Hex("flow_id", logging.ID(id.NodeID)).Str("role", id.Role.String()).Msg("pruned peer")
	t.metrics.OnLocalMeshSizeUpdated(topic, len(t.topicMeshMap[topic]))
}

// logLoop logs the mesh peers of the local node for each topic at a regular interval.
func (t *GossipSubMeshTracer) logLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	ticker := time.NewTicker(t.loggerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.logPeers()
		}
	}
}

// logPeers logs the mesh peers of the local node for each topic.
// Note that based on GossipSub parameters, we expect to have between 6 and 12 peers in the mesh for each topic.
// Hence, choosing a heartbeat interval in the order of minutes should be sufficient to log the mesh peers of the local node.
// Also, note that the mesh peers are also logged reactively when a peer is added or removed from the mesh.
func (t *GossipSubMeshTracer) logPeers() {
	t.topicMeshMu.RLock()
	defer t.topicMeshMu.RUnlock()

	for topic := range t.topicMeshMap {
		shouldWarn := false // whether we should warn about the mesh state
		lg := t.logger.With().Dur("heartbeat_interval", t.loggerInterval).Str("topic", topic).Logger()
		for p := range t.topicMeshMap[topic] {
			id, exists := t.idProvider.ByPeerID(p)
			if !exists {
				shouldWarn = true
				lg = lg.With().
					Str("peer_id", p.String()).
					Str("flow_id", "unknown").
					Str("role", "unknown").
					Logger()

				continue
			}

			lg = lg.With().
				Str("peer_id", p.String()).
				Hex("flow_id", logging.ID(id.NodeID)).
				Str("role", id.Role.String()).
				Logger()
		}

		if shouldWarn {
			lg.Warn().
				Bool(logging.KeySuspicious, true).
				Msg("topic mesh peers of local node since last heartbeat")
			continue
		}
		lg.Info().Msg("topic mesh peers of local node since last heartbeat")
	}
}

func (t *GossipSubMeshTracer) AddPeer(p peer.ID, proto protocol.ID) {
	// no-op
}

func (t *GossipSubMeshTracer) RemovePeer(p peer.ID) {
	// no-op
}

func (t *GossipSubMeshTracer) Join(topic string) {
	// no-op
}

func (t *GossipSubMeshTracer) Leave(topic string) {
	// no-op
}

func (t *GossipSubMeshTracer) ValidateMessage(msg *pubsub.Message) {
	// no-op
}

func (t *GossipSubMeshTracer) DeliverMessage(msg *pubsub.Message) {
	// no-op
}

func (t *GossipSubMeshTracer) RejectMessage(msg *pubsub.Message, reason string) {
	// no-op
}

func (t *GossipSubMeshTracer) DuplicateMessage(msg *pubsub.Message) {
	// no-op
}

func (t *GossipSubMeshTracer) ThrottlePeer(p peer.ID) {
	// no-op
}

func (t *GossipSubMeshTracer) RecvRPC(rpc *pubsub.RPC) {
	// no-op
}

func (t *GossipSubMeshTracer) SendRPC(rpc *pubsub.RPC, p peer.ID) {
	// no-op
}

func (t *GossipSubMeshTracer) DropRPC(rpc *pubsub.RPC, p peer.ID) {
	// no-op
}

func (t *GossipSubMeshTracer) UndeliverableMessage(msg *pubsub.Message) {
	// no-op
}
