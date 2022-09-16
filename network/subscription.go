package network

import "github.com/onflow/flow-go/network/channels"

type SubscriptionManager interface {
	// Register registers an engine on the channel into the subscription manager.
	Register(channel channels.Channel, engine MessageProcessor) error

	// Unregister removes the engine associated with a channel.
	Unregister(channel channels.Channel) error

	// GetEngine returns engine associated with a channel.
	GetEngine(channel channels.Channel) (MessageProcessor, error)

	// Channels returns all the channels registered in this subscription manager.
	Channels() channels.ChannelList
}
