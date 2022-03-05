package validator

import (
	"context"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/onflow/flow-go/model/flow"
)

func StakedValidator(getIdentity func(peer.ID) (*flow.Identity, bool)) MessageValidator {
	return func(ctx context.Context, from peer.ID, msg interface{}) pubsub.ValidationResult {
		if _, ok := getIdentity(from); ok {
			return pubsub.ValidationAccept
		}
		return pubsub.ValidationReject
	}
}
