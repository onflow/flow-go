package p2p

import (
	"errors"
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
)

// SubscriptionProvider provides a list of topics a peer is subscribed to.
type SubscriptionProvider interface {
	component.Component
	// GetSubscribedTopics returns all the subscriptions of a peer within the pubsub network.
	// Note that the current peer must be subscribed to the topic for it to the same topics in order
	// to query for other peers, e.g., if current peer has subscribed to topics A and B, and peer1
	// has subscribed to topics A, B, and C, then GetSubscribedTopics(peer1) will return A and B. Since this peer
	// has not subscribed to topic C, it will not be able to query for other peers subscribed to topic C.
	GetSubscribedTopics(pid peer.ID) []string
}

// SubscriptionValidator validates the subscription of a peer to a topic.
// It is used to ensure that a peer is only subscribed to topics that it is allowed to subscribe to.
type SubscriptionValidator interface {
	component.Component
	// CheckSubscribedToAllowedTopics checks if a peer is subscribed to topics that it is allowed to subscribe to.
	// Args:
	// 	pid: the peer ID of the peer to check
	//  role: the role of the peer to check
	// Returns:
	// error: if the peer is subscribed to topics that it is not allowed to subscribe to, an InvalidSubscriptionError is returned.
	// The error is benign, i.e., it does not indicate an illegal state in the execution of the code. We expect this error
	// when there are malicious peers in the network. But such errors should not lead to a crash of the node.
	CheckSubscribedToAllowedTopics(pid peer.ID, role flow.Role) error
}

// TopicProvider provides a low-level abstraction for pubsub to perform topic-related queries.
// This abstraction is provided to encapsulate the pubsub implementation details from the rest of the codebase.
type TopicProvider interface {
	// GetTopics returns all the topics within the pubsub network that the current peer has subscribed to.
	GetTopics() []string

	// ListPeers returns all the peers subscribed to a topic.
	// Note that the current peer must be subscribed to the topic for it to query for other peers.
	// If the current peer is not subscribed to the topic, an empty list is returned.
	// For example, if current peer has subscribed to topics A and B, then ListPeers only return
	// subscribed peers for topics A and B, and querying for topic C will return an empty list.
	ListPeers(topic string) []peer.ID
}

// InvalidSubscriptionError indicates that a peer has subscribed to a topic that is not allowed for its role.
// This error is benign, i.e., it does not indicate an illegal state in the execution of the code. We expect this error
// when there are malicious peers in the network. But such errors should not lead to a crash of the node.32
type InvalidSubscriptionError struct {
	topic string // the topic that the peer is subscribed to, but not allowed to.
}

func NewInvalidSubscriptionError(topic string) error {
	return InvalidSubscriptionError{
		topic: topic,
	}
}

func (e InvalidSubscriptionError) Error() string {
	return fmt.Sprintf("unauthorized subscription: %s", e.topic)
}

func IsInvalidSubscriptionError(this error) bool {
	var e InvalidSubscriptionError
	return errors.As(this, &e)
}
