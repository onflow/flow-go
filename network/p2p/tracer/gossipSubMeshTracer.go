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
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/p2plogging"
	"github.com/onflow/flow-go/network/p2p/tracer/internal"
	"github.com/onflow/flow-go/utils/logging"
)

const (
	// MeshLogIntervalMsg is the message logged by the tracer every logInterval.
	MeshLogIntervalMsg = "topic mesh peers of local node since last heartbeat"

	// MeshLogIntervalWarnMsg is the message logged by the tracer every logInterval if there are unknown peers in the mesh.
	MeshLogIntervalWarnMsg = "unknown peers in topic mesh peers of local node since last heartbeat"

	// defaultLastHighestIHaveRPCSizeResetInterval is the interval that we reset the tracker of max ihave size sent back
	// to a default. We use ihave message max size to determine the health of requested iwants from remote peers. However,
	// we don't desire an ihave size anomaly to persist forever, hence, we reset it back to a default every minute.
	// The choice of the interval to be a minute is in harmony with the GossipSub decay interval.
	defaultLastHighestIHaveRPCSizeResetInterval = time.Minute
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

	topicMeshMu                  sync.RWMutex                    // to protect topicMeshMap
	topicMeshMap                 map[string]map[peer.ID]struct{} // map of local mesh peers by topic.
	logger                       zerolog.Logger
	idProvider                   module.IdentityProvider
	loggerInterval               time.Duration
	metrics                      module.GossipSubLocalMeshMetrics
	rpcSentTracker               *internal.RPCSentTracker
	duplicateMessageTrackerCache *internal.GossipSubDuplicateMessageTrackerCache
}

var _ p2p.PubSubTracer = (*GossipSubMeshTracer)(nil)

type GossipSubMeshTracerConfig struct {
	network.NetworkingType
	metrics.HeroCacheMetricsFactory
	Logger                             zerolog.Logger
	Metrics                            module.GossipSubLocalMeshMetrics
	IDProvider                         module.IdentityProvider
	LoggerInterval                     time.Duration
	RpcSentTrackerCacheSize            uint32
	RpcSentTrackerWorkerQueueCacheSize uint32
	RpcSentTrackerNumOfWorkers         int
	DuplicateMessageTrackerCacheSize   uint32
	DuplicateMessageTrackerGuageDecay  float64
}

// NewGossipSubMeshTracer creates a new *GossipSubMeshTracer.
// Args:
// - *GossipSubMeshTracerConfig: the mesh tracer config.
// Returns:
// - *GossipSubMeshTracer: new mesh tracer.
func NewGossipSubMeshTracer(config *GossipSubMeshTracerConfig) *GossipSubMeshTracer {
	lg := config.Logger.With().Str("component", "gossipsub_topology_tracer").Logger()
	rpcSentTracker := internal.NewRPCSentTracker(&internal.RPCSentTrackerConfig{
		Logger:                             lg,
		RPCSentCacheSize:                   config.RpcSentTrackerCacheSize,
		RPCSentCacheCollector:              metrics.GossipSubRPCSentTrackerMetricFactory(config.HeroCacheMetricsFactory, config.NetworkingType),
		WorkerQueueCacheCollector:          metrics.GossipSubRPCSentTrackerQueueMetricFactory(config.HeroCacheMetricsFactory, config.NetworkingType),
		WorkerQueueCacheSize:               config.RpcSentTrackerWorkerQueueCacheSize,
		NumOfWorkers:                       config.RpcSentTrackerNumOfWorkers,
		LastHighestIhavesSentResetInterval: defaultLastHighestIHaveRPCSizeResetInterval,
	})
	g := &GossipSubMeshTracer{
		RawTracer:      NewGossipSubNoopTracer(),
		topicMeshMap:   make(map[string]map[peer.ID]struct{}),
		idProvider:     config.IDProvider,
		metrics:        config.Metrics,
		logger:         lg,
		loggerInterval: config.LoggerInterval,
		rpcSentTracker: rpcSentTracker,
		duplicateMessageTrackerCache: internal.NewGossipSubDuplicateMessageTrackerCache(
			config.DuplicateMessageTrackerCacheSize,
			config.DuplicateMessageTrackerGuageDecay,
			config.Logger,
			metrics.GossipSubDuplicateMessageTrackerCacheMetricFactory(config.HeroCacheMetricsFactory, config.NetworkingType),
		),
	}

	g.Component = component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()
			g.logLoop(ctx)
		}).
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()
			lg.Debug().Msg("starting rpc sent tracker")
			g.rpcSentTracker.Start(ctx)
			lg.Debug().Msg("rpc sent tracker started")

			<-g.rpcSentTracker.Done()
			lg.Debug().Msg("rpc sent tracker stopped")
		}).
		Build()

	return g
}

// GetLocalMeshPeers returns the local mesh peers for the given topic.
// Args:
// - topic: the topic.
// Returns:
// - []peer.ID: the local mesh peers for the given topic.
func (t *GossipSubMeshTracer) GetLocalMeshPeers(topic channels.Topic) []peer.ID {
	t.topicMeshMu.RLock()
	defer t.topicMeshMu.RUnlock()

	peers := make([]peer.ID, 0, len(t.topicMeshMap[topic.String()]))
	for p := range t.topicMeshMap[topic.String()] {
		peers = append(peers, p)
	}
	return peers
}

// Graft is called when a peer is added to a topic mesh. The tracer uses this to track the mesh peers.
func (t *GossipSubMeshTracer) Graft(p peer.ID, topic string) {
	t.topicMeshMu.Lock()
	defer t.topicMeshMu.Unlock()

	lg := t.logger.With().Str("topic", topic).Str("peer_id", p2plogging.PeerId(p)).Logger()

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

	lg.Debug().Hex("flow_id", logging.ID(id.NodeID)).Str("role", id.Role.String()).Msg("grafted peer")
}

// Prune is called when a peer is removed from a topic mesh. The tracer uses this to track the mesh peers.
func (t *GossipSubMeshTracer) Prune(p peer.ID, topic string) {
	t.topicMeshMu.Lock()
	defer t.topicMeshMu.Unlock()

	lg := t.logger.With().Str("topic", topic).Str("peer_id", p2plogging.PeerId(p)).Logger()

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

	lg.Debug().Hex("flow_id", logging.ID(id.NodeID)).Str("role", id.Role.String()).Msg("pruned peer")
}

// SendRPC is called when an RPC is sent. Currently, the GossipSubMeshTracer tracks iHave RPC messages that have been sent.
// This function can be updated to track other control messages in the future as required.
func (t *GossipSubMeshTracer) SendRPC(rpc *pubsub.RPC, _ peer.ID) {
	err := t.rpcSentTracker.Track(rpc)
	if err != nil {
		t.logger.Err(err).Bool(logging.KeyNetworkingSecurity, true).Msg("failed to track sent pubsbub rpc")
	}
}

// DuplicateMessage is invoked when a duplicate message is dropped.
func (t *GossipSubMeshTracer) DuplicateMessage(msg *pubsub.Message) {
	peerID := msg.ReceivedFrom
	count, err := t.duplicateMessageTrackerCache.Inc(msg.ReceivedFrom)
	if err != nil {
		t.logger.Err(err).
			Bool(logging.KeyNetworkingSecurity, true).
			Str("peer_id", peerID.String()).
			Msg("failed to increment gossipsub duplicate message tracker count for peer")
		return
	}
	t.logger.Debug().
		Str("peer_id", peerID.String()).
		Float64("duplicate_message_count", count).
		Msg("gossipsub duplicate message tracker incremented for peer")
}

// WasIHaveRPCSent returns true if an iHave control message for the messageID was sent, otherwise false.
func (t *GossipSubMeshTracer) WasIHaveRPCSent(messageID string) bool {
	return t.rpcSentTracker.WasIHaveRPCSent(messageID)
}

// LastHighestIHaveRPCSize returns the last highest RPC iHave message sent.
func (t *GossipSubMeshTracer) LastHighestIHaveRPCSize() int64 {
	return t.rpcSentTracker.LastHighestIHaveRPCSize()
}

// DuplicateMessageCount returns the current duplicate message count for the peer.
func (t *GossipSubMeshTracer) DuplicateMessageCount(peerID peer.ID) float64 {
	count, err := t.duplicateMessageTrackerCache.Get(peerID)
	if err != nil {
		t.logger.Err(err).
			Bool(logging.KeyNetworkingSecurity, true).
			Str("peer_id", peerID.String()).
			Msg("failed to get duplicate message count for peer")
		return 0
	}
	return count
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
				topicPeers = topicPeers.Str(strconv.Itoa(peerIndex), fmt.Sprintf("pid=%s, flow_id=unknown, role=unknown", p2plogging.PeerId(p)))
				continue
			}

			topicPeers = topicPeers.Str(strconv.Itoa(peerIndex), fmt.Sprintf("pid=%s, flow_id=%x, role=%s", p2plogging.PeerId(p), id.NodeID, id.Role.String()))
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
		lg.Debug().Msg(MeshLogIntervalMsg)
	}
}
