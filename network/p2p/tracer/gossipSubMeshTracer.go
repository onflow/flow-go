package tracer

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/tracer/internal"
	"github.com/onflow/flow-go/utils/logging"
)

const (
	// MeshLogIntervalMsg is the message logged by the tracer every logInterval.
	MeshLogIntervalMsg = "topic mesh peers of local node since last heartbeat"

	// MeshLogIntervalWarnMsg is the message logged by the tracer every logInterval if there are unknown peers in the mesh.
	MeshLogIntervalWarnMsg = "unknown peers in topic mesh peers of local node since last heartbeat"
)

// The GossipSubMeshTracer component in the GossipSub pubsub.RawTracer that is designed to track the local
// mesh peers for each topic. By logging the mesh peers and updating the local mesh size metric, the GossipSubMeshTracer
// provides insights into the behavior of the topology.
//
// This component also provides real-time and historical visibility into the topology.
// The GossipSubMeshTracer logs the mesh peers of the local node for each topic
// at a regular interval, enabling users to monitor the state of the mesh network and take appropriate action.
// Additionally, it allows users to configure the logging interval.
type GossipSubMeshTracer struct {
	component.Component
	pubsub.RawTracer

	topicMeshMu    sync.RWMutex                    // to protect topicMeshMap
	topicMeshMap   map[string]map[peer.ID]struct{} // map of local mesh peers by topic.
	logger         zerolog.Logger
	idProvider     module.IdentityProvider
	loggerInterval time.Duration
	metrics        module.GossipSubLocalMeshMetrics
	rpcSentTracker *internal.RPCSentTracker
}

var _ p2p.PubSubTracer = (*GossipSubMeshTracer)(nil)

type GossipSubMeshTracerConfig struct {
	Logger                       zerolog.Logger
	Metrics                      module.GossipSubLocalMeshMetrics
	IDProvider                   module.IdentityProvider
	LoggerInterval               time.Duration
	RpcSentTrackerCacheCollector module.HeroCacheMetrics
	RpcSentTrackerCacheSize      uint32
}

func NewGossipSubMeshTracer(config *GossipSubMeshTracerConfig) (*GossipSubMeshTracer, error) {
	rpcSentTracker, err := internal.NewRPCSentTracker(config.Logger, config.RpcSentTrackerCacheSize, config.RpcSentTrackerCacheCollector)
	if err != nil {
		return nil, err
	}

	g := &GossipSubMeshTracer{
		RawTracer:      NewGossipSubNoopTracer(),
		topicMeshMap:   make(map[string]map[peer.ID]struct{}),
		idProvider:     config.IDProvider,
		metrics:        config.Metrics,
		logger:         config.Logger.With().Str("component", "gossip_sub_topology_tracer").Logger(),
		loggerInterval: config.LoggerInterval,
		rpcSentTracker: rpcSentTracker,
	}

	g.Component = component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()
			g.logLoop(ctx)
		}).
		Build()

	return g, nil
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
	meshSize := len(t.topicMeshMap[topic])

	t.metrics.OnLocalMeshSizeUpdated(topic, meshSize)
	lg = lg.With().Int("mesh_size", meshSize).Logger()

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

	meshSize := len(t.topicMeshMap[topic])
	t.metrics.OnLocalMeshSizeUpdated(topic, meshSize)
	lg = lg.With().Int("mesh_size", meshSize).Logger()

	id, exists := t.idProvider.ByPeerID(p)
	if !exists {
		lg.Warn().
			Bool(logging.KeySuspicious, true).
			Msg("pruned peer not found in identity provider")

		return
	}

	lg.Info().Hex("flow_id", logging.ID(id.NodeID)).Str("role", id.Role.String()).Msg("pruned peer")
}

// SendRPC is called when a RPC is sent. Currently, the GossipSubMeshTracer tracks iHave RPC messages that have been sent.
// This function can be updated to track other control messages in the future as required.
func (t *GossipSubMeshTracer) SendRPC(rpc *pubsub.RPC, _ peer.ID) {
	switch {
	case len(rpc.GetControl().GetIhave()) > 0:
		t.rpcSentTracker.OnIHaveRPCSent(rpc.GetControl().GetIhave())
	}
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
	defer t.topicMeshMu.RUnlock()
	for topic := range t.topicMeshMap {
		shouldWarn := false // whether we should warn about the mesh state

		topicPeers := zerolog.Dict()

		peerIndex := -1 // index to keep track of peer info in different logging dictionaries.
		for p := range t.topicMeshMap[topic] {
			peerIndex++
			id, exists := t.idProvider.ByPeerID(p)

			if !exists {
				shouldWarn = true
				topicPeers = topicPeers.Str(strconv.Itoa(peerIndex), fmt.Sprintf("pid=%s, flow_id=unknown, role=unknown", p.String()))
				continue
			}

			topicPeers = topicPeers.Str(strconv.Itoa(peerIndex), fmt.Sprintf("pid=%s, flow_id=%x, role=%s", p.String(), id.NodeID, id.Role.String()))
		}

		lg := t.logger.With().
			Dur("heartbeat_interval", t.loggerInterval).
			Str("topic", topic).
			Dict("topic_mesh", topicPeers).
			Logger()

		if shouldWarn {
			lg.Warn().
				Bool(logging.KeySuspicious, true).
				Msg(MeshLogIntervalWarnMsg)
			continue
		}
		lg.Info().Msg(MeshLogIntervalMsg)
	}
}
