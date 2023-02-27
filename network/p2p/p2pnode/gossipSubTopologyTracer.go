package p2pnode

import (
	"sync"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/utils/logging"
)

type GossipSubMeshTracer struct {
	topicMeshMu    sync.RWMutex                    // to protect topicMeshMap
	topicMeshMap   map[string]map[peer.ID]struct{} // map of local mesh peers by topic.
	logger         zerolog.Logger
	idProvider     module.IdentityProvider
	loggerInterval time.Duration
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

// logLoop logs the mesh peers of the local node for each topic at a regular interval.
func (t *GossipSubMeshTracer) logLoop(ctx irrecoverable.SignalerContext) {
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
	defer t.topicMeshMu.Unlock()

	for topic := range t.topicMeshMap {
		lg := t.logger.With().Dur("heartbeat_interval", t.loggerInterval).Str("topic", topic).Logger()
		for p := range t.topicMeshMap[topic] {
			id, exists := t.idProvider.ByPeerID(p)
			if !exists {
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

		lg.Info().Msg("topic mesh peers of local node since last heartbeat")
	}
}
