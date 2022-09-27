package scoring

import (
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/network/p2p"
)

type SubscriptionProvider struct {
	logger       zerolog.Logger
	mu           sync.RWMutex
	tp           p2p.TopicProvider
	peersByTopic map[string][]peer.ID
	allTopics    []string
}

func NewSubscriptionProvider(logger zerolog.Logger, tp p2p.TopicProvider) *SubscriptionProvider {
	return &SubscriptionProvider{
		logger:       logger.With().Str("module", "subscription_provider").Logger(),
		tp:           tp,
		peersByTopic: make(map[string][]peer.ID),
		allTopics:    make([]string, 0),
	}
}

// GetSubscribedTopics returns all the subscriptions of a peer within the pubsub network.
func (s *SubscriptionProvider) GetSubscribedTopics(pid peer.ID) []string {
	topics := s.getAllTopics()
	subscriptions := make([]string, 0)
	for _, topic := range topics {
		peers := s.getPeersByTopic(topic)
		for _, p := range peers {
			if p == pid {
				subscriptions = append(subscriptions, topic)
			}
		}
	}

	return subscriptions
}

func (s *SubscriptionProvider) getAllTopics() []string {
	go func() {
		allTopics := s.tp.GetTopics()

		s.mu.Lock()
		s.allTopics = allTopics
		s.mu.Unlock()

		s.logger.Info().Msgf("all topics updated: %v", allTopics)
	}()

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.allTopics
}

func (s *SubscriptionProvider) getPeersByTopic(topic string) []peer.ID {
	go func() {
		subscribedPeers := s.tp.ListPeers(topic)

		s.mu.Lock()
		s.peersByTopic[topic] = subscribedPeers
		s.mu.Unlock()

		s.logger.Info().Str("topic", topic).Msgf("peers by topic updated: %v", subscribedPeers)
	}()

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.peersByTopic[topic]
}
