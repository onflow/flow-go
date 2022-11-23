package corruptnet

import (
	"context"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/connection"
	"github.com/onflow/flow-go/network/p2p/p2pnode"
	"github.com/onflow/flow-go/network/p2p/unicast"
	validator "github.com/onflow/flow-go/network/validator/pubsub"
)

// AcceptAllTopicValidator pubsub validator func that does not perform any validation, it will only attempt to decode the message and update the
// rawMsg.ValidatorData needed for further processing by the middleware receive loop. Malformed messages that fail to unmarshal or decode will result
// in a pubsub.ValidationReject result returned.
func AcceptAllTopicValidator(lg zerolog.Logger, c network.Codec) p2p.TopicValidatorFunc {
	lg = lg.With().
		Str("component", "corrupted_libp2pnode_topic_validator").
		Logger()

	return func(ctx context.Context, from peer.ID, rawMsg *pubsub.Message) p2p.ValidationResult {
		var msg message.Message
		err := msg.Unmarshal(rawMsg.Data)
		if err != nil {
			lg.Err(err).Msg("could not unmarshal raw message data")
			return p2p.ValidationReject
		}

		decodedMsgPayload, err := c.Decode(msg.Payload)
		if err != nil {
			lg.Err(err).Msg("could not decode message payload")
			return p2p.ValidationReject
		}

		rawMsg.ValidatorData = validator.TopicValidatorData{
			Message:           &msg,
			DecodedMsgPayload: decodedMsgPayload,
			From:              from,
		}

		return p2p.ValidationAccept
	}
}

// CorruptP2PNode is a wrapper around the original LibP2P node.
type CorruptP2PNode struct {
	*p2pnode.Node
	logger zerolog.Logger
	codec  network.Codec
}

// Subscribe subscribes the node to the given topic with a noop topic validator.
// All errors returned from this function can be considered benign.
func (n *CorruptP2PNode) Subscribe(topic channels.Topic, _ p2p.TopicValidatorFunc) (p2p.Subscription, error) {
	topicValidator := AcceptAllTopicValidator(n.logger, n.codec)
	return n.Node.Subscribe(topic, topicValidator)
}

// NewCorruptLibP2PNode returns corrupted libP2PNode that will subscribe to topics using the AcceptAllTopicValidator.
func NewCorruptLibP2PNode(logger zerolog.Logger, host host.Host, pCache *p2pnode.ProtocolPeerCache, uniMgr *unicast.Manager, peerManager *connection.PeerManager) p2p.LibP2PNode {
	node := p2pnode.NewNode(logger, host, pCache, uniMgr, peerManager)
	return &CorruptP2PNode{Node: node, logger: logger, codec: cbor.NewCodec()}
}
