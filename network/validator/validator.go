package validator

import (
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/network/p2p"
)

// MessageValidator validates the given message with original sender `from` and returns an error if validation fails
// else upon successful validation it should return the decoded message type string.
// Note: contrarily to pubsub.ValidatorEx, the peerID parameter does not represent the bearer of the message, but its source.
type MessageValidator func(from peer.ID, msg interface{}) (string, error)

// PubSubMessageValidator validates the given message with original sender `from` and returns pubsub.ValidationResult.
// Note: contrarily to pubsub.ValidatorEx, the peerID parameter does not represent the bearer of the message, but its source.
type PubSubMessageValidator func(from peer.ID, msg interface{}) p2p.ValidationResult
