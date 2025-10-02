package engine

// Notifier is a concurrency primitive for informing worker routines about the
// arrival of new work unit(s). Notifiers essentially behave like
// channels in that they can be passed by value and still allow concurrent
// updates of the same internal state.
//
// CAUTION: While individual notification send/receive operations are atomic, the handoff from the
// sender to the receiver is not. This means that when Notify() is called concurrently or in fast
// succession, some notifications may be dropped even if there are consumers waiting on the Channel().
// The Notifier should only be used for usecases that can tolerate dropped notifications.
type Notifier struct {
	// Illustrative description of the Notifier:
	// * When the gate is activated, it will let a _single_  person step through the gate.
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
	//
	// CAUTION: the handoff of non-blocking writes and reads on a channel are NOT atomic operations.
	// The order of concurrent reads and non-blocking writes is entirely dependent on the go scheduler.
	// Consider the following scenario:
	// - One or more routines are blocked on the channel
	// - One or more routines are producing notifications in fast succession (or concurrently)
	// Intuitively, you might expect that if there is a waiting consumer, then the consumer will take
	// the notification as soon as it is available. This is true when performing a blocking write,
	// however, when performing a non-blocking write (i.e. with a select/default statement), the read
	// happens when the go scheduler executes the routine. This means that the producer could fill
	// the buffered channel and exercise the default bypass while one or more consumers are waiting
	// to receive notifications.
	// This can be problematic when using the notifier to communicate work for a pool of workers.
	// In some situations, only a subset of workers are unblocked even if there is enough work for
	// all workers.

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
