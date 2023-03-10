package p2p

import (
	"context"
	"fmt"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"

	"github.com/onflow/flow-go/module/component"
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
}

// PubSubAdapterConfig abstracts the configuration for the underlying pubsub implementation.
type PubSubAdapterConfig interface {
	WithRoutingDiscovery(routing.ContentRouting)
	WithSubscriptionFilter(SubscriptionFilter)
	WithScoreOption(ScoreOptionBuilder)
	WithMessageIdFunction(f func([]byte) string)
	WithAppSpecificRpcInspector(f func(peer.ID, *pubsub.RPC) error)
	WithTracer(t PubSubTracer)

	// WithScoreTracer sets the tracer for the underlying pubsub score implementation.
	// This is used to expose the local scoring table of the GossipSub node to its higher level components.
	WithScoreTracer(tracer PeerScoreTracer)
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
	// BuildFlowPubSubScoreOption builds the pubsub score options as pubsub.Option for the Flow network.
	BuildFlowPubSubScoreOption() pubsub.Option
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
}

// PeerScoreSnapshot is a snapshot of the overall peer score at a given time.
type PeerScoreSnapshot struct {
	Score              float64
	Topics             map[string]*TopicScoreSnapshot
	AppSpecificScore   float64
	IPColocationFactor float64
	BehaviourPenalty   float64
}

// TopicScoreSnapshot is a snapshot of the peer score within a topic at a given time.
type TopicScoreSnapshot struct {
	TimeInMesh               time.Duration
	FirstMessageDeliveries   float64
	MeshMessageDeliveries    float64
	InvalidMessageDeliveries float64
}

// IsWarning returns true if the peer score is in warning state.
func (p PeerScoreSnapshot) IsWarning() bool {
	for _, topic := range p.Topics {
		if topic.IsWarning() {
			return true
		}
	}

	if p.AppSpecificScore < 0 {
		return true
	}

	if p.IPColocationFactor > 0 {
		return true
	}

	if p.BehaviourPenalty > 0 {
		return true
	}

	if p.Score < 0 {
		return true
	}

	return false
}

func (s TopicScoreSnapshot) String() string {
	return fmt.Sprintf("time_in_mesh: %s, first_message_deliveries: %f, mesh message deliveries: %f, invalid message deliveries: %f",
		s.TimeInMesh, s.FirstMessageDeliveries, s.MeshMessageDeliveries, s.InvalidMessageDeliveries)
}

// IsWarning returns true if the topic score is in warning state.
func (s TopicScoreSnapshot) IsWarning() bool {
	// TODO: also check for first message deliveries and time in mesh when we have a better understanding of the score.
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
