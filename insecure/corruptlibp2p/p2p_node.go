package corruptlibp2p

import (
	"context"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/node"
	validator "github.com/onflow/flow-go/network/validator/pubsub"
)

// AcceptAllTopicValidator pubsub validator func that does not perform any validation, it will only attempt to decode the message and update the
// rawMsg.ValidatorData needed for further processing by the middleware receive loop. Malformed messages that fail to unmarshal or decode will result
// in a pubsub.ValidationReject result returned.
func AcceptAllTopicValidator() p2p.TopicValidatorFunc {
	return func(_ context.Context, from peer.ID, rawMsg *pubsub.Message) p2p.ValidationResult {
		var msg message.Message
		err := msg.Unmarshal(rawMsg.Data)
		if err != nil {
			return p2p.ValidationReject
		}

		rawMsg.ValidatorData = validator.TopicValidatorData{
			Message: &msg,
			From:    from,
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
	topicValidator := AcceptAllTopicValidator()
	return n.Node.Subscribe(topic, topicValidator)
}
