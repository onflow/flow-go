package mempool

import "github.com/onflow/flow-go/engine"

// EntityRequestStore is a FIFO (first-in-first-out) size-bound queue for maintaining EntityRequests.
// It is designed to be utilized at the common ProviderEngine to maintain and respond entity requests.
type EntityRequestStore interface {
	engine.MessageStore

	// Size returns total requests stored in queue.
	Size() uint
}
