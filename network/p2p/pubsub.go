package p2p

import (
	"context"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
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

type PubSubTracer interface {
	pubsub.RawTracer
}
