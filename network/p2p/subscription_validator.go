package p2p

import (
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/onflow/flow-go/module/id"
)

type SubscriptionValidator struct {
	idProvider           id.IdentityProvider
	subscriptionProvider SubscriptionProvider
}

func NewSubscriptionValidator(idProvider id.IdentityProvider) *SubscriptionValidator {
	return &SubscriptionValidator{
		idProvider: idProvider,
	}
}

func (v *SubscriptionValidator) RegisterSubscriptionProvider(provider SubscriptionProvider) {
	v.subscriptionProvider = provider
}

// ValidationSubscriptions validates all subscriptions a peer has with respect to all Flow topics.
// It returns true if the peer is allowed to subscribe to all topics that it has subscribed.
func (v *SubscriptionValidator) ValidationSubscriptions(pid peer.ID) bool {
	topics := v.subscriptionProvider.GetSubscribedTopics(pid)

	flowId, ok := v.idProvider.ByPeerID(pid)
	if !ok {
		return false
	}

	for _, topic := range topics {
		if AllowedSubscription(flowId.Role, topic) {
			return false
		}
	}

	return true
}
