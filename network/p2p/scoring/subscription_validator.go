package scoring

import (
	"fmt"

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
func (v *SubscriptionValidator) MustSubscribedToAllowedTopics(pid peer.ID) error {
	topics := v.subscriptionProvider.GetSubscribedTopics(pid)

	flowId, ok := v.idProvider.ByPeerID(pid)
	if !ok {
		return fmt.Errorf("could not find authorized identity for peer id %s", pid)
	}

	for _, topic := range topics {
		if !p2putils.AllowedSubscription(flowId.Role, topic) {
			return fmt.Errorf("unauthorized subscription: %s", topic)
		}
	}

	return nil
}
