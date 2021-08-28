package p2p

import (
	"context"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/onflow/flow-go/model/flow"
)

type StakedValidator struct {
	stakedIdentities func() flow.IdentityList
}

func (v *StakedValidator) Validate(ctx context.Context, receivedFrom peer.ID, msg *pubsub.Message) pubsub.ValidationResult {
	// check that message contains a valid sender ID
	if from, err := messageSigningID(msg); err == nil {
		// check that the sender peer ID can be converted to a Flow key
		if key, err := FlowPublicKeyFromPeerID(from); err == nil {
			// check that the Flow key belongs to a staked node
			if _, found := v.stakedIdentities().ByNetworkingKey(key); found {
				return pubsub.ValidationAccept
			}
		}
	}

	return pubsub.ValidationReject
}
