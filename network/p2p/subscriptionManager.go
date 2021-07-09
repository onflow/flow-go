package p2p

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/network"
)

// ChannelSubscriptionManager manages subscriptions of engines running on the node to channels.
// Each channel should be taken by at most a single engine.
type ChannelSubscriptionManager struct {
	mu      sync.RWMutex
	engines map[network.Channel][]network.Engine
	mw      network.Middleware
}

func NewChannelSubscriptionManager(mw network.Middleware) *ChannelSubscriptionManager {
	return &ChannelSubscriptionManager{
		engines: make(map[network.Channel][]network.Engine),
		mw:      mw,
	}
}

// Register registers an engine on the channel into the subscription manager.
func (sm *ChannelSubscriptionManager) Register(channel network.Channel, engine network.Engine) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	_, ok := sm.engines[channel]
	if !ok {
		// registers the channel with the middleware to let middleware start receiving messages
		err := sm.mw.Subscribe(channel)
		if err != nil {
			return fmt.Errorf("subscriptionManager: failed to subscribe to channel %s: %w", channel, err)
		}

		// initializes the engine set for the provided channel
		sm.engines[channel] = make([]network.Engine, 0)
	}

	sm.engines[channel] = append(sm.engines[channel], engine)

	return nil
}

// Unregister removes the engine associated with a channel.
func (sm *ChannelSubscriptionManager) Unregister(channel network.Channel) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// check if there is a registered engine for the given channel
	_, ok := sm.engines[channel]
	if !ok {
		// if not found then there is nothing else to do
		return nil
	}

	err := sm.mw.Unsubscribe(channel)
	if err != nil {
		return fmt.Errorf("subscriptionManager: failed to unregister from channel %s", channel)
	}

	delete(sm.engines, channel)

	return nil
}

// GetEngines returns the engines associated with a channel.
func (sm *ChannelSubscriptionManager) GetEngines(channel network.Channel) ([]network.Engine, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	engines, found := sm.engines[channel]
	if !found {
		return nil, fmt.Errorf("subscriptionManager: engine for channel %s not found", channel)
	}
	return engines, nil
}

// Channels returns all the channels registered in this subscription manager.
func (sm *ChannelSubscriptionManager) Channels() network.ChannelList {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	channels := make(network.ChannelList, 0)
	for channel := range sm.engines {
		channels = append(channels, channel)
	}

	return channels
}
