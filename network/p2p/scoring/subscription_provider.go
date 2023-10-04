package scoring

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/p2pconf"
)

// SubscriptionProvider provides a list of topics a peer is subscribed to.
type SubscriptionProvider struct {
	component.Component
	logger zerolog.Logger
	tp     p2p.TopicProvider

	// allTopics is a list of all topics in the pubsub network
	// TODO: we should add an expiry time to this cache and clean up the cache periodically
	// to avoid leakage of stale topics.
	peersByTopic         sync.Map      // map[topic]peers
	peersByTopicUpdating sync.Map      // whether a goroutine is already updating the list of peers for a topic
	peerUpdateInterval   time.Duration // the interval for updating the list of peers for a topic.

	// allTopics is a list of all topics in the pubsub network that this node is subscribed to.
	allTopicsLock           sync.RWMutex  // protects allTopics
	allTopics               []string      // list of all topics in the pubsub network that this node has subscribed to.
	allTopicsUpdate         atomic.Bool   // whether a goroutine is already updating the list of topics
	allTopicsUpdateInterval time.Duration // the interval for updating the list of topics in the pubsub network that this node has subscribed to.
}

type SubscriptionProviderConfig struct {
	Logger        zerolog.Logger    `validate:"required"`
	TopicProvider p2p.TopicProvider `validate:"required"`
	Params        *p2pconf.SubscriptionProviderParameters
}

func NewSubscriptionProvider(cfg *SubscriptionProviderConfig) (*SubscriptionProvider, error) {
	if err := validator.New().Struct(cfg); err != nil {
		return nil, fmt.Errorf("invalid subscription provider config: %w", err)
	}

	p := &SubscriptionProvider{
		logger:                  cfg.Logger.With().Str("module", "subscription_provider").Logger(),
		tp:                      cfg.TopicProvider,
		allTopics:               make([]string, 0),
		allTopicsUpdateInterval: cfg.Params.SubscriptionUpdateInterval,
		peerUpdateInterval:      cfg.Params.PeerSubscriptionUpdateTTL,
	}

	builder := component.NewComponentManagerBuilder()
	builder.AddWorker(
		func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()
			p.logger.Debug().Msg("subscription provider started; starting update topics loop")
			p.updateTopicsLoop(ctx)

			<-ctx.Done()
			p.logger.Debug().Msg("subscription provider stopped; stopping update topics loop")
		})

	return p, nil
}

// GetSubscribedTopics returns all the subscriptions of a peer within the pubsub network.
// Note that the current node can only see peer subscriptions to topics that it has also subscribed to
// e.g., if current node has subscribed to topics A and B, and peer1 has subscribed to topics A, B, and C,
// then GetSubscribedTopics(peer1) will return A and B. Since this node has not subscribed to topic C,
// it will not be able to query for other peers subscribed to topic C.
func (s *SubscriptionProvider) GetSubscribedTopics(pid peer.ID) []string {
	// finds the topics that this peer is subscribed to.
	subscriptions := make([]string, 0)
	for _, topic := range s.getDeepCopyAllTopics() {
		peers := s.getPeersByTopic(topic)
		for _, p := range peers {
			if p == pid {
				subscriptions = append(subscriptions, topic)
			}
		}
	}

	return subscriptions
}

func (s *SubscriptionProvider) updateTopicsLoop(ctx irrecoverable.SignalerContext) {
	ticker := time.NewTicker(s.allTopicsUpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.updateTopics()
		}
	}
}

// updateTopics returns all the topics in the pubsub network that this node (peer) has subscribed to.
// Note that this method always returns the cached version of the subscribed topics while querying the
// pubsub network for the list of topics in a goroutine. Hence, the first call to this method always returns an empty
// list.
func (s *SubscriptionProvider) updateTopics() {
	if updateInProgress := s.allTopicsUpdate.CompareAndSwap(false, true); updateInProgress {
		// another goroutine is already updating the list of topics
		return
	}

	allTopics := s.tp.GetTopics()
	s.atomicUpdateAllTopics(allTopics)

	// remove the update flag
	s.allTopicsUpdate.Store(false)

	s.logger.Trace().Msgf("all topics updated: %v", allTopics)
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

// getDeepCopyAllTopics returns a deep copy of the list of all topics in the pubsub network that this node has subscribed to.
func (s *SubscriptionProvider) getDeepCopyAllTopics() []string {
	s.allTopicsLock.RLock()
	defer s.allTopicsLock.RUnlock()

	allTopics := make([]string, len(s.allTopics))
	copy(allTopics, s.allTopics)
	return allTopics
}
