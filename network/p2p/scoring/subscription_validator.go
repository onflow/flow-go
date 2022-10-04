package scoring

import (
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network/p2p"
	p2putils "github.com/onflow/flow-go/network/p2p/utils"
)

type SubscriptionValidator struct {
	idProvider           module.IdentityProvider
	subscriptionProvider p2p.SubscriptionProvider
}

func NewSubscriptionValidator(idProvider module.IdentityProvider) *SubscriptionValidator {
	return &SubscriptionValidator{
		idProvider: idProvider,
	}
}

func (v *SubscriptionValidator) RegisterSubscriptionProvider(provider p2p.SubscriptionProvider) {
	v.subscriptionProvider = provider
}

// MustSubscribedToAllowedTopics validates all subscriptions a peer has with respect to all Flow topics.
// All errors returned by this method are benign:
// We expect an unauthorized or ejected peer to subscribe to Flow topics (InvalidPeerIDError).
// We expect a peer to subscribe to a topic that is not allowed for its role (InvalidSubscriptionError).
func (v *SubscriptionValidator) MustSubscribedToAllowedTopics(pid peer.ID) error {
	topics := v.subscriptionProvider.GetSubscribedTopics(pid)

	flowId, ok := v.idProvider.ByPeerID(pid)
	if !ok {
		return NewInvalidPeerIDError(pid, PeerIdStatusUnknown)
	}

	if flowId.Ejected {
		return NewInvalidPeerIDError(pid, PeerIdStatusEjected)
	}

	for _, topic := range topics {
		if !p2putils.AllowedSubscription(flowId.Role, topic) {
			return NewInvalidSubscriptionError(topic)
		}
	}

	return nil
}
