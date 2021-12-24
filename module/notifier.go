package module

// Notifier is a concurrency primitive for informing worker routines about the
// arrival of new work unit(s). Notifiers essentially behave like
// channels in that they can be passed by value and still allow concurrent
// updates of the same internal state.
type Notifier struct {
	// Illustrative description of the Notifier:
	// * When the gate is activate, it will let a _single_  person step through the gate.
	// * When somebody steps through the gate, it deactivates (atomic operation) and
	//   prevents subsequent people from passing (until it is activated again).
	// * The gate has a memory and remembers whether it is activated. I.e. the gate
	//   can be activated while no-one is waiting. When a person arrives later, they
	//   can pass through the gate.
	// * Activating an already-activated gate is a no-op.
	//
	// Implementation:
	// We implement the Notifier using a channel. Activating the gate corresponds to
	// calling `Notify()` on the Notifier, which pushes an element to the channel.
	// Passing through the gate corresponds to receiving from the `Channel()`.
	// As we don't want the routine sending the notification to wait until a worker
	// routine reads from the channel, we need a buffered channel with capacity 1.

	notifier chan struct{} // buffered channel with capacity 1
}

// NewNotifier instantiates a Notifier. Notifiers essentially behave like
// channels in that they can be passed by value and still allow concurrent
// updates of the same internal state.
func NewNotifier() Notifier {
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
