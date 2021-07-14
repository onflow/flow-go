package network

import (
	"errors"
	"fmt"
	"sync"

	"github.com/onflow/flow-go/engine/common/splitter"
	"github.com/onflow/flow-go/network"
)

// ChannelSubscriptionManager manages subscriptions of engines running on the node to channels.
// Each channel should be taken by at most a single engine.
type SubscriptionManager struct {
	mu        sync.RWMutex
	splitters map[network.Channel]*splitter.Engine
}

func NewSubscriptionManager() *SubscriptionManager {
	return &SubscriptionManager{
		splitters: make(map[network.Channel]*splitter.Engine),
	}
}

// Register registers an engine on the channel into the subscription manager.
func (sm *SubscriptionManager) Register(channel network.Channel, engine network.Engine) error {
	splitter, ok := engine.(*splitter.Engine)

	if !ok {
		return errors.New("engine is not a splitter")
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, ok := sm.splitters[channel]; ok {
		return fmt.Errorf("channel already registered: %s", channel)
	}

	sm.splitters[channel] = splitter

	return nil
}

// Unregister removes the engine associated with a channel.
func (sm *SubscriptionManager) Unregister(channel network.Channel) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	delete(sm.splitters, channel)

	return nil
}

// GetEngine returns engine associated with a channel.
func (sm *SubscriptionManager) GetEngine(channel network.Channel) (network.Engine, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	splitter, found := sm.splitters[channel]

	if !found {
		return nil, fmt.Errorf("splitter for channel %s not found", channel)
	}

	return splitter, nil
}

// Channels returns all the channels registered in this subscription manager.
func (sm *SubscriptionManager) Channels() network.ChannelList {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	channels := make(network.ChannelList, 0)
	for channel := range sm.splitters {
		channels = append(channels, channel)
	}

	return channels
}
