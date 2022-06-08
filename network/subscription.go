package network

type SubscriptionManager interface {
	// Register registers an engine on the channel into the subscription manager.
	Register(channel Channel, engine MessageProcessor) error

	// Unregister removes the engine associated with a channel.
	Unregister(channel Channel) error

	// GetEngine returns engine associated with a channel.
	GetEngine(channel Channel) (MessageProcessor, error)

	// Channels returns all the channels registered in this subscription manager.
	Channels() ChannelList
}
