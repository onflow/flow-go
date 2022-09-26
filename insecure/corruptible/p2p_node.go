package corruptible

import (
	"context"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p/p2pnode"
)

// AcceptAllTopicValidator pubsub validator func that always returns pubsub.ValidationAccept.
func AcceptAllTopicValidator(context.Context, peer.ID, *pubsub.Message) pubsub.ValidationResult {
	return pubsub.ValidationAccept
}

// Node is a wrapper around the original LibP2P node.
type Node struct {
	*p2pnode.Node
}

// Subscribe subscribes the node to the given topic with a noop topic validator.
// The following benign errors are expected during normal operations from libP2P:
//   - topic cannot be subscribed to
// All errors returned from this function can be considered benign.
func (n *Node) Subscribe(topic channels.Topic, _ pubsub.ValidatorEx) (*pubsub.Subscription, error) {
	return n.Node.Subscribe(topic, AcceptAllTopicValidator)
}
