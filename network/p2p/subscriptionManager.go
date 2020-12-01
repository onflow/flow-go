package p2p

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/network"
)

// ChannelSubscriptionManager manages the engine to channelID subscription
type ChannelSubscriptionManager struct {
	mu      sync.RWMutex
	engines map[string]network.Engine
	mw      network.Middleware
}

func NewChannelSubscriptionManager(mw network.Middleware) *ChannelSubscriptionManager {
	return &ChannelSubscriptionManager{
		engines: make(map[string]network.Engine),
		mw:      mw,
	}
}

func (sm *ChannelSubscriptionManager) Register(channelID string, engine network.Engine) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

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

func (sm *ChannelSubscriptionManager) Unregister(channelID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

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

func (sm *ChannelSubscriptionManager) GetEngine(channelID string) (network.Engine, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	eng, found := sm.engines[channelID]
	if !found {
		return nil, fmt.Errorf("subscriptionManager: engine for channelID %s not found", channelID)
	}
	return eng, nil
}

// GetChannelIDs returns list of topics this subscription manager has an engine registered for.
func (sm *ChannelSubscriptionManager) GetChannelIDs() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	topics := make([]string, 0)
	for topic := range sm.engines {
		topics = append(topics, topic)
	}

	return topics
}
