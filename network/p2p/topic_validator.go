package p2p

import (
	"context"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/onflow/flow-go/model/flow"
)

type StakedValidator struct {
	getIdentity func(peer.ID) (*flow.Identity, bool)
}

func (v *StakedValidator) Validate(ctx context.Context, receivedFrom peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
	// check that message contains a valid sender ID
	if from, err := messageSigningID(msg); err == nil {
		// check that the sender peer ID matches a staked Flow key
		if _, exists := v.getIdentity(from); exists {
			return pubsub.ValidationAccept
		}
	}

	return pubsub.ValidationReject
}
