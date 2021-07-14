package network

type SubscriptionManager interface {
	// Register registers a message processor on the channel into the subscription manager.
	Register(channel Channel, messageProcessor MessageProcessor) error

	// Unregister removes the message processor associated with a channel.
	Unregister(channel Channel) error

	// GetMessageProcessor returns message processor associated with a channel.
	GetMessageProcessor(channel Channel) (MessageProcessor, error)

	// Channels returns all the channels registered in this subscription manager.
	Channels() ChannelList
}
