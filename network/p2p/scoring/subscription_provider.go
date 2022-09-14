package scoring

import (
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
)

type SubscriptionProvider struct {
	ps *pubsub.PubSub
}

func NewSubscriptionProvider(ps *pubsub.PubSub) *SubscriptionProvider {
	return &SubscriptionProvider{
		ps: ps,
	}
}

// GetSubscribedTopics returns all the subscriptions of a peer within the pubsub network.
func (s *SubscriptionProvider) GetSubscribedTopics(pid peer.ID) []string {
	topics := s.ps.GetTopics()
	subscriptions := make([]string, 0)
	for _, topic := range topics {
		peers := s.ps.ListPeers(topic)
		for _, p := range peers {
			if p == pid {
				subscriptions = append(subscriptions, topic)
			}
		}
	}

	return subscriptions
}
