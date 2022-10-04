package scoring

import (
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/network/p2p"
)

// SubscriptionProvider provides a list of topics a peer is subscribed to.
type SubscriptionProvider struct {
	logger zerolog.Logger
	tp     p2p.TopicProvider

	// allTopics is a list of all topics in the pubsub network
	// TODO: we should add an expiry time to this cache and clean up the cache periodically
	// to avoid leakage of stale topics.
	peersByTopic         sync.Map // map[topic]peers
	peersByTopicUpdating sync.Map // whether a goroutine is already updating the list of peers for a topic

	// allTopics is a list of all topics in the pubsub network that this node is subscribed to.
	allTopicsLock   sync.RWMutex // protects allTopics
	allTopics       []string     // list of all topics in the pubsub network that this node has subscribed to.
	allTopicsUpdate atomic.Bool  // whether a goroutine is already updating the list of topics.
}

func NewSubscriptionProvider(logger zerolog.Logger, tp p2p.TopicProvider) *SubscriptionProvider {
	return &SubscriptionProvider{
		logger:    logger.With().Str("module", "subscription_provider").Logger(),
		tp:        tp,
		allTopics: make([]string, 0),
	}
}

// GetSubscribedTopics returns all the subscriptions of a peer within the pubsub network.
// Note that the current node can only see peer subscriptions to topics that it has also subscribed to
// e.g., if current node has subscribed to topics A and B, and peer1 has subscribed to topics A, B, and C,
// then GetSubscribedTopics(peer1) will return A and B. Since this node has not subscribed to topic C,
// it will not be able to query for other peers subscribed to topic C.
func (s *SubscriptionProvider) GetSubscribedTopics(pid peer.ID) []string {
	topics := s.getAllTopics()

	// finds the topics that this peer is subscribed to.
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

// getAllTopics returns all the topics in the pubsub network that this node (peer) has subscribed to.
// Note that this method always returns the cached version of the subscribed topics while querying the
// pubsub network for the list of topics in a goroutine. Hence, the first call to this method always returns an empty
// list.
func (s *SubscriptionProvider) getAllTopics() []string {
	go func() {
		// TODO: refactor this to a component manager worker once we have a startable libp2p node.
		if updateInProgress := s.allTopicsUpdate.CompareAndSwap(false, true); updateInProgress {
			// another goroutine is already updating the list of topics
			return
		}

		allTopics := s.tp.GetTopics()
		s.atomicUpdateAllTopics(allTopics)

		// remove the update flag
		s.allTopicsUpdate.Store(false)

		s.logger.Trace().Msgf("all topics updated: %v", allTopics)
	}()

	s.allTopicsLock.RLock()
	defer s.allTopicsLock.RUnlock()
	return s.allTopics
}

// getPeersByTopic returns all the peers subscribed to a topic.
// Note that this method always returns the cached version of the subscribed peers while querying the
// pubsub network for the list of topics in a goroutine. Hence, the first call to this method always returns an empty
// list.
// As this method is injected into GossipSub, it is vital that it never block the caller, otherwise it causes a
// deadlock on the GossipSub.
// Also note that, this peer itself should be subscribed to the topic, otherwise, it cannot find the list of peers
// subscribed to the topic in the pubsub network due to an inherent limitation of GossipSub.
func (s *SubscriptionProvider) getPeersByTopic(topic string) []peer.ID {
	go func() {
		// TODO: refactor this to a component manager worker once we have a startable libp2p node.
		if _, updateInProgress := s.peersByTopicUpdating.LoadOrStore(topic, true); updateInProgress {
			// another goroutine is already updating the list of peers for this topic
			return
		}

		subscribedPeers := s.tp.ListPeers(topic)
		s.peersByTopic.Store(topic, subscribedPeers)

		// remove the update flag
		s.peersByTopicUpdating.Delete(topic)

		s.logger.Trace().Str("topic", topic).Msgf("peers by topic updated: %v", subscribedPeers)
	}()

	peerId, ok := s.peersByTopic.Load(topic)
	if !ok {
		return make([]peer.ID, 0)
	}
	return peerId.([]peer.ID)
}

// atomicUpdateAllTopics updates the list of all topics in the pubsub network that this node has subscribed to.
func (s *SubscriptionProvider) atomicUpdateAllTopics(allTopics []string) {
	s.allTopicsLock.Lock()
	s.allTopics = allTopics
	s.allTopicsLock.Unlock()
}
