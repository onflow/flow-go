package engine

import (
	"sync"
)

// Trapdoor is a concurrency primitive to sleep worker routines if no work is available. A routine
// providing work activates the trapdoor. Repeated activation of an already activated trapdoor are
// no-ops. Once activated, a single routine can pass the trapdoor and the trapdoor deactivates again.
// If multiple routines are blocked on the trapdoor, each call to `Activate()` will wake up a single
// routine.
//
// Activation and passing of the trapdoor are atomic operations.
// Trapdoor establishes a happens-before relationship: the completion of the `Activate()` call happens
// before the channel returned by `Channel()` is closed.
//
// Trapdoor is distinct from Notifier in that the send/receive handoff is atomic. This means that if
// there are many blocked consumers, every call to `Activate()` will wake up a single consumer,
// regardless of the order in which goroutines are executed by the go scheduler.
//
// Trapdoor is concurrency safe.
//
// CAUTION: Trapdoor must not be copied or passed by value.
type Trapdoor struct {
	_ sync.Mutex // prevent copying of the struct

	activated bool
	shutdown  bool
	cond      *sync.Cond
}

// NewTrapdoor creates a new instance of a Trapdoor
// The returned trapdoor must be closed with Close() to avoid leaking goroutines.
// CAUTION: Trapdoor must not be copied or passed by value.
func NewTrapdoor() *Trapdoor {
	return &Trapdoor{
		cond: sync.NewCond(&sync.Mutex{}),
	}
}

// Close shuts down goroutines used by the trapdoor.
// After `Close()` is called, any new or existing channels returned by `Channel()` will never be closed,
// and subsequent calls to `Activate()` will cause a panic.
// This method is idempotent. Multiple calls to `Close()` are no-ops.
func (td *Trapdoor) Close() {
	td.cond.L.Lock()
	defer td.cond.L.Unlock()
	if !td.shutdown {
		td.shutdown = true
		td.cond.Broadcast() // wake up all blocked routines
	}
}

// Activate activates the trapdoor and wakes a single worker.
//
// Calls to `Activate()` after `Close()` will panic.
func (td *Trapdoor) Activate() {
	td.cond.L.Lock()
	defer td.cond.L.Unlock()

	if td.shutdown {
		panic("call to trapdoor.Activate() after close")
	}

	// always send the signal, even if the trapdoor was already activated. This guarantees that if
	// there are blocked workers, each call to `Activate()` will wake up a single worker.
	// `cond.Signal()` is a no-op if no routines are waiting.
	td.activated = true
	td.cond.Signal()
}

// pass blocks the calling routine if the trapdoor is not activated or shutdown, otherwise it returns
// immediately. The trapdoor is deactivated after passing. Returns false if the trapdoor is shutdown.
func (td *Trapdoor) pass() bool {
	td.cond.L.Lock()
	defer td.cond.L.Unlock()

	if td.shutdown {
		return false
	}

	if !td.activated {
		td.cond.Wait() // only return when awoken by Broadcast or Signal
	}
	td.activated = false

	// since cond.Wait() internally releases the lock, `shutdown` may have changed since we last checked.
	return !td.shutdown
}

// Channel returns a channel that is closed when the trapdoor is activated.
// Each call to `Channel()` returns a new channel.
//
// IMPORTANT: `Channel()` must be called for each wait. Since the returned channel is closed to signal
// that the trapdoor is activated, it cannot be reused for subsequent notifications.
func (td *Trapdoor) Channel() <-chan struct{} {
	ch := make(chan struct{})
	started := make(chan struct{})
	go func() {
		close(started)

		if td.pass() {
			close(ch)
		}
	}()
	<-started
	return ch
}
