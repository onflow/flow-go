package libp2p

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/gossip/libp2p/middleware"
)

// subscriptionManager manages the engine to channelID subscription
type subscriptionManager struct {
	sync.RWMutex
	engines map[string]network.Engine
	mw      middleware.Middleware
}

func newSubscriptionManager(mw middleware.Middleware) *subscriptionManager {
	return &subscriptionManager{
		engines: make(map[string]network.Engine),
		mw:      mw,
	}
}

func (sm *subscriptionManager) register(channelID string, engine network.Engine) error {
	sm.Lock()
	defer sm.Unlock()

	// check if the engine engineID is already taken
	_, ok := sm.engines[channelID]
	if ok {
		return fmt.Errorf("subscriptionManager: channel already registered: %s", channelID)
	}

	// register the channel ID with the middleware to start receiving messages
	err := sm.mw.Subscribe(channelID)
	if err != nil {
		return fmt.Errorf("subscriptionManager: failed to subscribe to channel %s: %w", channelID, err)
	}

	// save the engine for the provided channelID
	sm.engines[channelID] = engine

	return nil
}

func (sm *subscriptionManager) unregister(channelID string) error {
	sm.Lock()
	defer sm.Unlock()

	// check if there is a registered engine for the given channelID
	_, ok := sm.engines[channelID]
	if !ok {
		// if not found then there is nothing else to do
		return nil
	}

	err := sm.mw.Unsubscribe(channelID)
	if err != nil {
		return fmt.Errorf("subscriptionManager: failed to unregister from channel %s", channelID)
	}

	delete(sm.engines, channelID)

	return nil
}

func (sm *subscriptionManager) getEngine(channelID string) (network.Engine, error) {
	sm.RLock()
	defer sm.RUnlock()
	eng, found := sm.engines[channelID]
	if !found {
		return nil, fmt.Errorf("subscriptionManager: engine for channelID %s not found", channelID)
	}
	return eng, nil
}

// registeredTopics returns list of topics this subscription manager has an engine registered for.
func (sm *subscriptionManager) registeredTopics() []string {
	topics := make([]string, 0)
	for topic := range sm.engines {
		topics = append(topics, topic)
	}

	return topics
}
