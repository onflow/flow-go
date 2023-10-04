package scoring

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/p2p"
	p2putils "github.com/onflow/flow-go/network/p2p/utils"
)

// SubscriptionValidator validates that a peer is subscribed to topics that it is allowed to subscribe to.
// It is used to penalize peers that subscribe to topics that they are not allowed to subscribe to in GossipSub.
type SubscriptionValidator struct {
	subscriptionProvider p2p.SubscriptionProvider
}

func NewSubscriptionValidator() *SubscriptionValidator {
	return &SubscriptionValidator{}
}

var _ p2p.SubscriptionValidator = (*SubscriptionValidator)(nil)

// RegisterSubscriptionProvider registers the subscription provider with the subscription validator.
// This follows a dependency injection pattern.
// Args:
//
//	provider: the subscription provider
//
// Returns:
//
//			error: if the subscription provider is nil, an error is returned. The error is irrecoverable, i.e.,
//	     it indicates an illegal state in the execution of the code. We expect this error only when there is a bug in the code.
//	     Such errors should lead to a crash of the node.
func (v *SubscriptionValidator) RegisterSubscriptionProvider(provider p2p.SubscriptionProvider) error {
	if v.subscriptionProvider != nil {
		return fmt.Errorf("subscription provider already registered")
	}
	v.subscriptionProvider = provider

	return nil
}

// CheckSubscribedToAllowedTopics checks if a peer is subscribed to topics that it is allowed to subscribe to.
// Args:
//
//		pid: the peer ID of the peer to check
//	 role: the role of the peer to check
//
// Returns:
// error: if the peer is subscribed to topics that it is not allowed to subscribe to, an InvalidSubscriptionError is returned.
// The error is benign, i.e., it does not indicate an illegal state in the execution of the code. We expect this error
// when there are malicious peers in the network. But such errors should not lead to a crash of the node.
func (v *SubscriptionValidator) CheckSubscribedToAllowedTopics(pid peer.ID, role flow.Role) error {
	topics := v.subscriptionProvider.GetSubscribedTopics(pid)

	for _, topic := range topics {
		if !p2putils.AllowedSubscription(role, topic) {
			return p2p.NewInvalidSubscriptionError(topic)
		}
	}

	return nil
}
