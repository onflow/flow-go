package validator

import (
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/p2p"
)

// PubSubMessageValidator validates the given message with original sender `from` and returns p2p.ValidationResult.
// Note: contrarily to pubsub.ValidatorEx, the peerID parameter does not represent the bearer of the message, but its source.
type PubSubMessageValidator func(from peer.ID, msg *message.Message) p2p.ValidationResult
