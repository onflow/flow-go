package tracer

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
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

	topicMeshMu    sync.RWMutex                    // to protect topicMeshMap
	topicMeshMap   map[string]map[peer.ID]struct{} // map of local mesh peers by topic.
	logger         zerolog.Logger
	idProvider     module.IdentityProvider
	loggerInterval time.Duration
	metrics        module.LocalGossipSubRouterMetrics
	rpcSentTracker *internal.RPCSentTracker
}

var _ p2p.PubSubTracer = (*GossipSubMeshTracer)(nil)

type GossipSubMeshTracerConfig struct {
	network.NetworkingType
	metrics.HeroCacheMetricsFactory
	Logger                             zerolog.Logger
	Metrics                            module.LocalGossipSubRouterMetrics
	IDProvider                         module.IdentityProvider
	LoggerInterval                     time.Duration
	RpcSentTrackerCacheSize            uint32
	RpcSentTrackerWorkerQueueCacheSize uint32
	RpcSentTrackerNumOfWorkers         int
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
		topicMeshMap:   make(map[string]map[peer.ID]struct{}),
		idProvider:     config.IDProvider,
		metrics:        config.Metrics,
		logger:         lg,
		loggerInterval: config.LoggerInterval,
		rpcSentTracker: rpcSentTracker,
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

// Graft is called by GossipSub when a peer is added to a topic mesh. The tracer uses this to track the mesh peers.
func (t *GossipSubMeshTracer) Graft(p peer.ID, topic string) {
	t.metrics.OnPeerGraftTopic(topic)
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

// Prune is called by GossipSub when a peer is removed from a topic mesh. The tracer uses this to track the mesh peers.
func (t *GossipSubMeshTracer) Prune(p peer.ID, topic string) {
	t.metrics.OnPeerPruneTopic(topic)
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

// SendRPC is called by GossipSub when a RPC is sent. Currently, the GossipSubMeshTracer tracks iHave RPC messages that have been sent.
// This function can be updated to track other control messages in the future as required.
func (t *GossipSubMeshTracer) SendRPC(rpc *pubsub.RPC, p peer.ID) {
	err := t.rpcSentTracker.Track(rpc)
	if err != nil {
		t.logger.Err(err).Bool(logging.KeyNetworkingSecurity, true).Msg("failed to track sent pubsbub rpc")
	}

	msgCount, ihaveCount, iwantCount, graftCount, pruneCount := 0, 0, 0, 0, 0
	if rpc.Control != nil {
		ihaveCount = len(rpc.Control.Ihave)
		iwantCount = len(rpc.Control.Iwant)
		graftCount = len(rpc.Control.Graft)
		pruneCount = len(rpc.Control.Prune)
	}
	msgCount = len(rpc.Publish)
	t.metrics.OnRpcReceived(msgCount, ihaveCount, iwantCount, graftCount, pruneCount)
	t.logger.Trace().
		Str("remote_peer_id", p2plogging.PeerId(p)).
		Int("subscription_option_count", len(rpc.Subscriptions)).
		Int("publish_message_count", msgCount).
		Int("ihave_size", ihaveCount).
		Int("iwant_size", iwantCount).
		Int("graft_size", graftCount).
		Int("prune_size", pruneCount).
		Msg("sent pubsub rpc")

	t.metrics.OnRpcSent(msgCount, ihaveCount, iwantCount, graftCount, pruneCount)
}

// AddPeer is called by GossipSub as a callback when a peer is added to the local node on a protocol, i.e., the local node is connected to the peer on a protocol.
// The peer may or may not be subscribed to any topic.
func (t *GossipSubMeshTracer) AddPeer(p peer.ID, proto protocol.ID) {
	t.logger.Trace().
		Str("local_peer_id", p2plogging.PeerId(p)).
		Str("protocol", string(proto)).
		Msg("peer added")
	t.metrics.OnPeerAddedToProtocol(string(proto))
}

// RemovePeer is called by GossipSub as a callback when a peer is removed from the local node,
// i.e., the local node is no longer connected to the peer.
func (t *GossipSubMeshTracer) RemovePeer(p peer.ID) {
	t.logger.Trace().
		Str("local_peer_id", p2plogging.PeerId(p)).
		Msg("peer removed")
	t.metrics.OnPeerRemovedFromProtocol()
}

// Join is called by GossipSub as a callback when the local node joins a topic.
func (t *GossipSubMeshTracer) Join(topic string) {
	t.logger.Trace().
		Str("topic", topic).
		Msg("local peer joined topic")
	t.metrics.OnLocalPeerJoinedTopic()
}

// Leave is called by GossipSub as a callback when the local node leaves a topic.
func (t *GossipSubMeshTracer) Leave(topic string) {
	t.logger.Trace().
		Str("topic", topic).
		Msg("local peer left topic")
	t.metrics.OnLocalPeerLeftTopic()
}

// ValidateMessage is called by GossipSub as a callback when a message is received by the local node and entered the validation phase.
// As the result of the validation, the message may be rejected or passed to the application (i.e., Flow protocol).
func (t *GossipSubMeshTracer) ValidateMessage(msg *pubsub.Message) {
	lg := t.logger.With().Logger()

	if msg.Topic != nil {
		lg = lg.With().Str("topic", *msg.Topic).Logger()
	}
	from, err := peer.IDFromBytes(msg.From)
	if err == nil {
		lg = lg.With().Str("remote_peer_id", p2plogging.PeerId(from)).Logger()
	}
	size := len(msg.Data)
	lg.Trace().
		Str("received_from", p2plogging.PeerId(msg.ReceivedFrom)).
		Int("message_size", size).
		Msg("received pubsub message entered validation phase")
	t.metrics.OnMessageEnteredValidation(size)
}

// DeliverMessage is called by GossipSub as a callback when the local node delivers a message to all subscribers of the topic.
func (t *GossipSubMeshTracer) DeliverMessage(msg *pubsub.Message) {
	lg := t.logger.With().Logger()

	if msg.Topic != nil {
		lg = lg.With().Str("topic", *msg.Topic).Logger()
	}
	from, err := peer.IDFromBytes(msg.From)
	if err == nil {
		lg = lg.With().Str("remote_peer_id", p2plogging.PeerId(from)).Logger()
	}
	size := len(msg.Data)
	lg.Trace().
		Str("received_from", p2plogging.PeerId(msg.ReceivedFrom)).
		Int("message_size", len(msg.Data)).
		Msg("delivered pubsub message to all subscribers")
	t.metrics.OnMessageDeliveredToAllSubscribers(size)
}

// RejectMessage is called by GossipSub as a callback when a message is rejected by the local node.
// The message may be rejected for a variety of reasons, but the most common reason is that the message is invalid with respect to signature.
// Any message that arrives at the local node should contain the peer id of the source (i.e., the peer that created the message), the
// networking public key of the source, and the signature of the message. The local node uses this information to verify the message.
// If any of the information is missing or invalid, the message is rejected.
func (t *GossipSubMeshTracer) RejectMessage(msg *pubsub.Message, reason string) {
	lg := t.logger.With().Logger()

	if msg.Topic != nil {
		lg = lg.With().Str("topic", *msg.Topic).Logger()
	}
	from, err := peer.IDFromBytes(msg.From)
	if err == nil {
		lg = lg.With().Str("remote_peer_id", p2plogging.PeerId(from)).Logger()
	}
	size := len(msg.Data)
	lg.Trace().
		Str("received_from", p2plogging.PeerId(msg.ReceivedFrom)).
		Int("message_size", size).
		Msg("rejected pubsub message")
	t.metrics.OnMessageRejected(size, reason)
}

// DuplicateMessage is called by GossipSub as a callback when a duplicate message is received by the local node.
func (t *GossipSubMeshTracer) DuplicateMessage(msg *pubsub.Message) {
	lg := t.logger.With().Logger()

	if msg.Topic != nil {
		lg = lg.With().Str("topic", *msg.Topic).Logger()
	}
	from, err := peer.IDFromBytes(msg.From)
	if err == nil {
		lg = lg.With().Str("remote_peer_id", p2plogging.PeerId(from)).Logger()
	}

	size := len(msg.Data)
	t.metrics.OnMessageDuplicate(size)
	lg.Trace().
		Str("received_from", p2plogging.PeerId(msg.ReceivedFrom)).
		Int("message_size", size).
		Msg("received duplicate pubsub message")
}

// ThrottlePeer is called by GossipSub when a peer is throttled by the local node, i.e., the local node is not accepting any
// pubsub message from the peer but may still accept control messages.
func (t *GossipSubMeshTracer) ThrottlePeer(p peer.ID) {
	t.logger.Warn().
		Bool(logging.KeyNetworkingSecurity, true).
		Str("remote_peer_id", p2plogging.PeerId(p)).
		Msg("throttled peer; no longer accepting pubsub messages from peer, but may still accept control messages")
	t.metrics.OnPeerThrottled()
}

// RecvRPC is called by GossipSub as a callback when an inbound RPC message is received by the local node,
// note that the RPC already passed the RPC inspection, hence its statistics may be different from the RPC inspector metrics, as
// the RPC inspector metrics are updated before the RPC inspection, and the RPC may gone through truncation or rejection.
// This callback tracks the RPC messages as they are completely received by the local GossipSub router.
func (t *GossipSubMeshTracer) RecvRPC(rpc *pubsub.RPC) {
	msgCount, ihaveCount, iwantCount, graftCount, pruneCount := 0, 0, 0, 0, 0
	if rpc.Control != nil {
		ihaveCount = len(rpc.Control.Ihave)
		iwantCount = len(rpc.Control.Iwant)
		graftCount = len(rpc.Control.Graft)
		pruneCount = len(rpc.Control.Prune)
	}
	msgCount = len(rpc.Publish)
	t.metrics.OnRpcReceived(msgCount, ihaveCount, iwantCount, graftCount, pruneCount)
	t.logger.Trace().
		Int("subscription_option_count", len(rpc.Subscriptions)).
		Int("publish_message_count", msgCount).
		Int("ihave_size", ihaveCount).
		Int("iwant_size", iwantCount).
		Int("graft_size", graftCount).
		Int("prune_size", pruneCount).
		Msg("received pubsub rpc")
}

// DropRPC is called by GossipSub as a callback when an outbound RPC message is dropped by the local node, typically because the local node
// outbound message queue is full; or the RPC is big and the local node cannot fragment it.
func (t *GossipSubMeshTracer) DropRPC(rpc *pubsub.RPC, p peer.ID) {
	msgCount, ihaveCount, iwantCount, graftCount, pruneCount := 0, 0, 0, 0, 0
	if rpc.Control != nil {
		ihaveCount = len(rpc.Control.Ihave)
		iwantCount = len(rpc.Control.Iwant)
		graftCount = len(rpc.Control.Graft)
		pruneCount = len(rpc.Control.Prune)
	}
	msgCount = len(rpc.Publish)
	t.metrics.OnRpcReceived(msgCount, ihaveCount, iwantCount, graftCount, pruneCount)
	t.logger.Warn().
		Bool(logging.KeyNetworkingSecurity, true).
		Str("remote_peer_id", p2plogging.PeerId(p)).
		Int("subscription_option_count", len(rpc.Subscriptions)).
		Int("publish_message_count", msgCount).
		Int("ihave_size", ihaveCount).
		Int("iwant_size", iwantCount).
		Int("graft_size", graftCount).
		Int("prune_size", pruneCount).
		Msg("outbound rpc dropped")

	t.metrics.OnOutboundRpcDropped()
}

// UndeliverableMessage is called by GossipSub as a callback when a message is dropped by the local node, typically because the local node
// outbound message queue is full; or the message is big and the local node cannot fragment it.
func (t *GossipSubMeshTracer) UndeliverableMessage(msg *pubsub.Message) {
	t.logger.Warn().
		Bool(logging.KeyNetworkingSecurity, true).
		Str("topic", *msg.Topic).
		Str("remote_peer_id", p2plogging.PeerId(msg.ReceivedFrom)).
		Int("message_size", len(msg.Data)).
		Msg("undeliverable pubsub message")
	t.metrics.OnUndeliveredMessage()
}

// WasIHaveRPCSent returns true if an iHave control message for the messageID was sent, otherwise false.
func (t *GossipSubMeshTracer) WasIHaveRPCSent(messageID string) bool {
	return t.rpcSentTracker.WasIHaveRPCSent(messageID)
}

// LastHighestIHaveRPCSize returns the last highest RPC iHave message sent.
func (t *GossipSubMeshTracer) LastHighestIHaveRPCSize() int64 {
	return t.rpcSentTracker.LastHighestIHaveRPCSize()
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
