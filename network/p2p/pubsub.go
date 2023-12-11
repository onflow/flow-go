package p2p

import (
	"context"
	"fmt"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"

	"github.com/onflow/flow-go/engine/collection"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/network/channels"
)

type ValidationResult int

const (
	ValidationAccept ValidationResult = iota
	ValidationIgnore
	ValidationReject
)

type TopicValidatorFunc func(context.Context, peer.ID, *pubsub.Message) ValidationResult

// PubSubAdapter is the abstraction of the underlying pubsub logic that is used by the Flow network.
type PubSubAdapter interface {
	component.Component
	// CollectionClusterChangesConsumer  is the interface for consuming the events of changes in the collection cluster.
	// This is used to notify the node of changes in the collection cluster.
	// PubSubAdapter implements this interface and consumes the events to be notified of changes in the clustering channels.
	// The clustering channels are used by the collection nodes of a cluster to communicate with each other.
	// As the cluster (and hence their cluster channels) of collection nodes changes over time (per epoch) the node needs to be notified of these changes.
	CollectionClusterChangesConsumer
	// RegisterTopicValidator registers a validator for topic.
	RegisterTopicValidator(topic string, topicValidator TopicValidatorFunc) error

	// UnregisterTopicValidator removes a validator from a topic.
	// Returns an error if there was no validator registered with the topic.
	UnregisterTopicValidator(topic string) error

	// Join joins the topic and returns a Topic handle.
	// Only one Topic handle should exist per topic, and Join will error if the Topic handle already exists.
	Join(topic string) (Topic, error)

	// GetTopics returns all the topics within the pubsub network that the current peer has subscribed to.
	GetTopics() []string

	// ListPeers returns all the peers subscribed to a topic.
	// Note that the current peer must be subscribed to the topic for it to query for other peers.
	// If the current peer is not subscribed to the topic, an empty list is returned.
	// For example, if current peer has subscribed to topics A and B, then ListPeers only return
	// subscribed peers for topics A and B, and querying for topic C will return an empty list.
	ListPeers(topic string) []peer.ID

	// GetLocalMeshPeers returns the list of peers in the local mesh for the given topic.
	// Args:
	// - topic: the topic.
	// Returns:
	// - []peer.ID: the list of peers in the local mesh for the given topic.
	GetLocalMeshPeers(topic channels.Topic) []peer.ID

	// PeerScoreExposer returns the peer score exposer for the gossipsub adapter. The exposer is a read-only interface
	// for querying peer scores and returns the local scoring table of the underlying gossipsub node.
	// The exposer is only available if the gossipsub adapter was configured with a score tracer.
	// If the gossipsub adapter was not configured with a score tracer, the exposer will be nil.
	// Args:
	//     None.
	// Returns:
	//    The peer score exposer for the gossipsub adapter.
	PeerScoreExposer() PeerScoreExposer
}

// PubSubAdapterConfig abstracts the configuration for the underlying pubsub implementation.
type PubSubAdapterConfig interface {
	WithRoutingDiscovery(routing.ContentRouting)
	WithSubscriptionFilter(SubscriptionFilter)
	WithScoreOption(ScoreOptionBuilder)
	WithMessageIdFunction(f func([]byte) string)
	WithTracer(t PubSubTracer)
	// WithScoreTracer sets the tracer for the underlying pubsub score implementation.
	// This is used to expose the local scoring table of the GossipSub node to its higher level components.
	WithScoreTracer(tracer PeerScoreTracer)
	WithInspectorSuite(GossipSubInspectorSuite)
}

// GossipSubControlMetricsObserver funcs used to observe gossipsub related metrics.
type GossipSubControlMetricsObserver interface {
	ObserveRPC(peer.ID, *pubsub.RPC)
}

// GossipSubRPCInspector app specific RPC inspector used to inspect and validate incoming RPC messages before they are processed by libp2p.
// Implementations must:
//   - be concurrency safe
//   - be non-blocking
type GossipSubRPCInspector interface {
	component.Component

	// Name returns the name of the rpc inspector.
	Name() string

	// Inspect inspects an incoming RPC message. This callback func is invoked
	// on ever RPC message received before the message is processed by libp2p.
	// If this func returns any error the RPC message will be dropped.
	Inspect(peer.ID, *pubsub.RPC) error
}

// GossipSubMsgValidationRpcInspector abstracts the general behavior of an app specific RPC inspector specifically
// used to inspect and validate incoming. It is used to implement custom message validation logic. It is injected into
// the GossipSubRouter and run on every incoming RPC message before the message is processed by libp2p. If the message
// is invalid the RPC message will be dropped.
// Implementations must:
//   - be concurrency safe
//   - be non-blocking
type GossipSubMsgValidationRpcInspector interface {
	collection.ClusterEvents
	GossipSubRPCInspector
}

// Topic is the abstraction of the underlying pubsub topic that is used by the Flow network.
type Topic interface {
	// String returns the topic name as a string.
	String() string

	// Close closes the topic.
	Close() error

	// Publish publishes a message to the topic.
	Publish(context.Context, []byte) error

	// Subscribe returns a subscription to the topic so that the caller can receive messages from the topic.
	Subscribe() (Subscription, error)
}

// ScoreOptionBuilder abstracts the configuration for the underlying pubsub score implementation.
type ScoreOptionBuilder interface {
	component.Component
	// BuildFlowPubSubScoreOption builds the pubsub score options as pubsub.Option for the Flow network.
	BuildFlowPubSubScoreOption() (*pubsub.PeerScoreParams, *pubsub.PeerScoreThresholds)
	// TopicScoreParams returns the topic score params for the given topic.
	// If the topic score params for the given topic does not exist, it will return the default topic score params.
	TopicScoreParams(*pubsub.Topic) *pubsub.TopicScoreParams
}

// Subscription is the abstraction of the underlying pubsub subscription that is used by the Flow network.
type Subscription interface {
	// Cancel cancels the subscription so that the caller will no longer receive messages from the topic.
	Cancel()

	// Topic returns the topic that the subscription is subscribed to.
	Topic() string

	// Next returns the next message from the subscription.
	Next(context.Context) (*pubsub.Message, error)
}

// BasePubSubAdapterConfig is the base configuration for the underlying pubsub implementation.
// These configurations are common to all pubsub implementations and must be observed by all implementations.
type BasePubSubAdapterConfig struct {
	// MaxMessageSize is the maximum size of a message that can be sent on the pubsub network.
	MaxMessageSize int
}

// SubscriptionFilter is the abstraction of the underlying pubsub subscription filter that is used by the Flow network.
type SubscriptionFilter interface {
	// CanSubscribe returns true if the current peer can subscribe to the topic.
	CanSubscribe(string) bool

	// FilterIncomingSubscriptions is invoked for all RPCs containing subscription notifications.
	// It filters and returns the subscriptions of interest to the current node.
	FilterIncomingSubscriptions(peer.ID, []*pb.RPC_SubOpts) ([]*pb.RPC_SubOpts, error)
}

// PubSubTracer is the abstraction of the underlying pubsub tracer that is used by the Flow network. It wraps the
// pubsub.RawTracer interface with the component.Component interface so that it can be started and stopped.
// The RawTracer interface is used to trace the internal events of the pubsub system.
type PubSubTracer interface {
	component.Component
	pubsub.RawTracer
	RpcControlTracking
	// DuplicateMessageCount returns the current duplicate message count for the peer.
	DuplicateMessageCount(peerID peer.ID) float64
	// GetLocalMeshPeers returns the list of peers in the mesh for the given topic.
	// Args:
	// - topic: the topic.
	// Returns:
	// - []peer.ID: the list of peers in the mesh for the given topic.
	GetLocalMeshPeers(topic channels.Topic) []peer.ID
}

// RpcControlTracking is the abstraction of the underlying libp2p control message tracker used to track message ids advertised by the iHave control messages.
// This collection of methods can ensure an iWant control message for a message-id corresponds to a broadcast iHave message id. Implementations
// must be non-blocking and concurrency safe.
type RpcControlTracking interface {
	// LastHighestIHaveRPCSize returns the last highest size of iHaves sent in a rpc.
	LastHighestIHaveRPCSize() int64
	// WasIHaveRPCSent checks if an iHave control message with the provided message ID was sent.
	WasIHaveRPCSent(messageID string) bool
}

// PeerScoreSnapshot is a snapshot of the overall peer score at a given time.
type PeerScoreSnapshot struct {
	// Score the overall score of the peer.
	Score float64
	// Topics map that stores the score of the peer per topic.
	Topics map[string]*TopicScoreSnapshot
	// AppSpecificScore application specific score (set by Flow protocol).
	AppSpecificScore float64

	// A positive value indicates that the peer is colocated with other nodes on the same network id,
	// and can be used to warn of sybil attacks.
	IPColocationFactor float64
	// A positive value indicates that GossipSub has caught the peer misbehaving, and can be used to warn of an attack.
	BehaviourPenalty float64
}

// TopicScoreSnapshot is a snapshot of the peer score within a topic at a given time.
// Note that float64 is used for the counters as they are decayed over the time.
type TopicScoreSnapshot struct {
	// TimeInMesh total time in mesh.
	TimeInMesh time.Duration
	// FirstMessageDeliveries counter of first message deliveries.
	FirstMessageDeliveries float64
	// MeshMessageDeliveries total mesh message deliveries (in the mesh).
	MeshMessageDeliveries float64
	// InvalidMessageDeliveries counter of invalid message deliveries.
	InvalidMessageDeliveries float64
}

// IsWarning returns true if the peer score is in warning state. When the peer score is in warning state, the peer is
// considered to be misbehaving.
func (p PeerScoreSnapshot) IsWarning() bool {
	// Check if any topic is in warning state.
	for _, topic := range p.Topics {
		if topic.IsWarning() {
			return true
		}
	}

	// Check overall score.
	switch {
	case p.Score < -1:
		// If the overall score is negative, the peer is in warning state, it means that the peer is suspected to be
		// misbehaving at the GossipSub level.
		return true
	// Check app-specific score.
	case p.AppSpecificScore < -1:
		// If the app specific score is negative, the peer is in warning state, it means that the peer behaves in a way
		// that is not allowed by the Flow protocol.
		return true
	// Check IP colocation factor.
	case p.IPColocationFactor > 5:
		// If the IP colocation factor is positive, the peer is in warning state, it means that the peer is running on the
		// same IP as another peer and is suspected to be a sybil node. For now, we set it to a high value to make sure
		// that peers from the same operator are not marked as sybil nodes.
		// TODO: this should be revisited once the collocation penalty is enabled.
		return true
	// Check behaviour penalty.
	case p.BehaviourPenalty > 20:
		// If the behaviour penalty is positive, the peer is in warning state, it means that the peer is suspected to be
		// misbehaving at the GossipSub level, e.g. sending too many duplicate messages. Setting it to 20 to reduce the noise; 20 is twice the threshold (defaultBehaviourPenaltyThreshold).
		return true
	// If none of the conditions are met, return false.
	default:
		return false
	}
}

// String returns the string representation of the peer score snapshot.
func (s TopicScoreSnapshot) String() string {
	return fmt.Sprintf("time_in_mesh: %s, first_message_deliveries: %f, mesh message deliveries: %f, invalid message deliveries: %f",
		s.TimeInMesh, s.FirstMessageDeliveries, s.MeshMessageDeliveries, s.InvalidMessageDeliveries)
}

// IsWarning returns true if the topic score is in warning state.
func (s TopicScoreSnapshot) IsWarning() bool {
	// TODO: also check for first message deliveries and time in mesh when we have a better understanding of the score.
	// If invalid message deliveries is positive, the topic is in warning state. It means that the peer is suspected to
	// be misbehaving at the GossipSub level, e.g. sending too many invalid messages to the topic.
	return s.InvalidMessageDeliveries > 0
}

// PeerScoreTracer is the interface for the tracer that is used to trace the peer score.
type PeerScoreTracer interface {
	component.Component
	PeerScoreExposer
	// UpdatePeerScoreSnapshots updates the peer score snapshot/
	UpdatePeerScoreSnapshots(map[peer.ID]*PeerScoreSnapshot)

	// UpdateInterval returns the update interval for the tracer. The tracer will be receiving updates
	// at this interval.
	UpdateInterval() time.Duration
}

// PeerScoreExposer is the interface for the tracer that is used to expose the peers score.
type PeerScoreExposer interface {
	// GetScore returns the overall score for the given peer.
	GetScore(peerID peer.ID) (float64, bool)
	// GetAppScore returns the application score for the given peer.
	GetAppScore(peerID peer.ID) (float64, bool)
	// GetIPColocationFactor returns the IP colocation factor for the given peer.
	GetIPColocationFactor(peerID peer.ID) (float64, bool)
	// GetBehaviourPenalty returns the behaviour penalty for the given peer.
	GetBehaviourPenalty(peerID peer.ID) (float64, bool)
	// GetTopicScores returns the topic scores for the given peer for all topics.
	// The returned map is keyed by topic name.
	GetTopicScores(peerID peer.ID) (map[string]TopicScoreSnapshot, bool)
}
