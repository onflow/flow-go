package mempool

// OnEjection is a callback which a mempool executes on ejecting
// one of its elements. The callbacks are executed from within the thread
// that serves the mempool. Implementations should be non-blocking.
type OnEjection[V any] func(V)
