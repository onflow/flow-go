package validator

import (
	"context"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/onflow/flow-go/network/message"
)

func NotSelfValidator(self peer.ID) MessageValidator {
	return func(ctx context.Context, from peer.ID, msg *message.Message) pubsub.ValidationResult {
		if from == self {
			return pubsub.ValidationIgnore
		}
		return pubsub.ValidationAccept
	}
}
