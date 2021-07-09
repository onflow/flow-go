package network

type SubscriptionManager interface {
	// Register registers an engine on the channel into the subscription manager.
	Register(channel Channel, engine Engine) error

	// Unregister removes the engine associated with a channel.
	Unregister(channel Channel) error

	// GetEngine returns the engines associated with a channel.
	GetEngines(channel Channel) ([]Engine, error)

	// Channels returns all the channels registered in this subscription manager.
	Channels() ChannelList
}
