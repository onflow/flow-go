package scoring

import (
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/onflow/flow-go/network/p2p"
)

type SubscriptionProvider struct {
	tp p2p.TopicProvider
}

func NewSubscriptionProvider(tp p2p.TopicProvider) *SubscriptionProvider {
	return &SubscriptionProvider{
		tp: tp,
	}
}

// GetSubscribedTopics returns all the subscriptions of a peer within the pubsub network.
func (s *SubscriptionProvider) GetSubscribedTopics(pid peer.ID) []string {
	topics := s.tp.GetTopics()
	subscriptions := make([]string, 0)
	for _, topic := range topics {
		peers := s.tp.ListPeers(topic)
		for _, p := range peers {
			if p == pid {
				subscriptions = append(subscriptions, topic)
			}
		}
	}

	return subscriptions
}
