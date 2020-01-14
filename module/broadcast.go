package module

// Subscription is a subscription to a particular event broadcast.
type Subscription interface {

	// Ch returns a channel that is sent over when a broadcast occurs.
	Ch() <-chan struct{}

	// Unsubscribe cancels this subscription. Once Unsubscribe is called, no
	// further notifications are sent over the channel.
	Unsubscribe()
}

// Broadcaster notifies subscribers when some event occurs.
type Broadcaster interface {

	// Subscribe adds a subscriber to the broadcast. Future broadcasts will
	// notify the new subscriptions channel.
	Subscribe() Subscription

	// Broadcast notifies all subscribers.
	Broadcast()
}
