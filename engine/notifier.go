package engine

// Notifier is a concurrency primitive for informing worker routines about the
// arrival of new work unit(s). Notifiers essentially behave like
// channels in that they can be passed by value and still allow concurrent
// updates of the same internal state.
type Notifier struct {
	notifier chan struct{}
}

// NewNotifier instantiates a Notifier. Notifiers essentially behave like
// channels in that they can be passed by value and still allow concurrent
// updates of the same internal state.
func NewNotifier() Notifier {
	// the 1 message buffer is important to avoid the race condition.
	// the consumer might decide to listen to the notify channel, and drain the messages in the
	// message store, however there is a blind period start from the point the consumer learned
	// the message store is empty to the point the consumer start listening to the notifier channel
	// again. During this blind period, if the notifier had no buffer, then `doNotify` call will not
	// able to push message to the notifier channel, therefore has to drop the message and cause the
	// consumer waiting forever with unconsumed message in the message store.
	// having 1 message buffer covers the "blind period", so that during the blind period if there is
	// a new message arrived, it will be buffered, and once the blind period is over, the consumer
	// will empty the buffer and start draining the message store again.

	return Notifier{make(chan struct{}, 1)}
}

// Notify sends a notification
func (n Notifier) Notify() {
	select {
	// to prevent from getting blocked by dropping the notification if
	// there is no handler subscribing the channel.
	case n.notifier <- struct{}{}:
	default:
	}
}

// Channel returns a channel for receiving notifications
func (n Notifier) Channel() <-chan struct{} {
	return n.notifier
}
