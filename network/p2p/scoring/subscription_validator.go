package scoring

import (
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/p2p"
	p2putils "github.com/onflow/flow-go/network/p2p/utils"
)

type SubscriptionValidator struct {
	subscriptionProvider p2p.SubscriptionProvider
}

func NewSubscriptionValidator() *SubscriptionValidator {
	return &SubscriptionValidator{}
}

func (v *SubscriptionValidator) RegisterSubscriptionProvider(provider p2p.SubscriptionProvider) {
	v.subscriptionProvider = provider
}

// CheckSubscribedToAllowedTopics validates all subscriptions a peer has with respect to all Flow topics.
// All errors returned by this method are benign:
// - InvalidSubscriptionError: the peer is subscribed to a topic that is not allowed for its role.
func (v *SubscriptionValidator) CheckSubscribedToAllowedTopics(pid peer.ID, role flow.Role) error {
	topics := v.subscriptionProvider.GetSubscribedTopics(pid)

	for _, topic := range topics {
		if !p2putils.AllowedSubscription(role, topic) {
			return NewInvalidSubscriptionError(topic)
		}
	}

	return nil
}
