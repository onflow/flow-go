package p2pnode

import (
	"sync"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/utils/logging"
)

type GossipSubMeshTracer struct {
	topicMeshMu  sync.Mutex                      // to protect topicMeshMap
	topicMeshMap map[string]map[peer.ID]struct{} // map of local mesh peers by topic.
	logger       zerolog.Logger
	idProvider   module.IdentityProvider

	pubsub.RawTracer
}

var _ pubsub.RawTracer = (*GossipSubMeshTracer)(nil)

func NewGossipSubTopologyTracer(logger zerolog.Logger, idProvider module.IdentityProvider) *GossipSubMeshTracer {
	return &GossipSubMeshTracer{
		topicMeshMap: make(map[string]map[peer.ID]struct{}),
		idProvider:   idProvider,
		logger:       logger.With().Str("component", "gossip_sub_topology_tracer").Logger(),
	}
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
}

