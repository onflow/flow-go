package corruptible

import (
	"context"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/connection"
	"github.com/onflow/flow-go/network/p2p/unicast"
	"github.com/rs/zerolog"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"

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
// All errors returned from this function can be considered benign.
func (n *P2PNode) Subscribe(topic channels.Topic, _ pubsub.ValidatorEx) (*pubsub.Subscription, error) {
	return n.Node.Subscribe(topic, AcceptAllTopicValidator)
}

func NewCorruptibleLibP2PNode(logger zerolog.Logger, host host.Host, pCache *p2pnode.ProtocolPeerCache, uniMgr *unicast.Manager, peerManager *connection.PeerManager) p2p.LibP2PNode {
	node := p2pnode.NewNode(logger, host, pCache, uniMgr, peerManager)
	return &P2PNode{node}
}
