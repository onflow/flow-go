package corruptible

import (
	"context"

	"github.com/rs/zerolog"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/onflow/flow-go/network/p2p/unicast"

	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p/p2pnode"
)

// AcceptAllTopicValidator pubsub validator func that always returns pubsub.ValidationAccept.
func AcceptAllTopicValidator(context.Context, peer.ID, *pubsub.Message) pubsub.ValidationResult {
	return pubsub.ValidationAccept
}

// P2PNode is a wrapper around the original LibP2P node.
type P2PNode struct {
	*p2pnode.Node
}

// Subscribe subscribes the node to the given topic with a noop topic validator.
// The following benign errors are expected during normal operations from libP2P:
//   - topic cannot be subscribed to
//
// All errors returned from this function can be considered benign.
func (n *P2PNode) Subscribe(topic channels.Topic, _ pubsub.ValidatorEx) (*pubsub.Subscription, error) {
	return n.Node.Subscribe(topic, AcceptAllTopicValidator)
}

// NewCorruptibleNode returns a corrupted libp2p node
func NewCorruptibleNode(logger zerolog.Logger,
	host host.Host,
	pubSub *pubsub.PubSub,
	routing routing.Routing,
	pCache *p2pnode.ProtocolPeerCache,
	uniMgr *unicast.Manager,
) p2pnode.LibP2PNode {
	return &P2PNode{p2pnode.NewNode(logger, host, pubSub, routing, pCache, uniMgr)}
}
