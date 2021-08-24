package converter

import "github.com/onflow/flow-go/network"

type SubscriptionManager struct {
	subMngr network.SubscriptionManager
	from    network.Channel
	to      network.Channel
}

func NewSubscriptionManager(subMngr network.SubscriptionManager, from network.Channel, to network.Channel) *SubscriptionManager {
	return &SubscriptionManager{subMngr, from, to}
}

func (sm *SubscriptionManager) convert(channel network.Channel) network.Channel {
	if channel == sm.from {
		return sm.to
	}
	return channel
}

func (sm *SubscriptionManager) reverse(channel network.Channel) network.Channel {
	if channel == sm.to {
		return sm.from
	}
	return channel
}

func (sm *SubscriptionManager) Register(channel network.Channel, engine network.Engine) error {
	return sm.subMngr.Register(sm.convert(channel), engine)
}

func (sm *SubscriptionManager) Unregister(channel network.Channel) error {
	return sm.subMngr.Unregister(sm.convert(channel))
}

func (sm *SubscriptionManager) GetEngine(channel network.Channel) (network.Engine, error) {
	return sm.subMngr.GetEngine(sm.convert(channel))
}

func (sm *SubscriptionManager) Channels() network.ChannelList {
	var channels network.ChannelList
	for _, ch := range sm.subMngr.Channels() {
		channels = append(channels, sm.reverse(ch))
	}
	return channels
}
