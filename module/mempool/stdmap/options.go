package stdmap

import (
	"github.com/onflow/flow-go/module/mempool"
)

// OptionFunc is a function that can be provided to the backend on creation in
// order to set a certain custom option.
type OptionFunc[K comparable, V any] func(backend *Backend[K, V])

// WithLimit can be provided to the backend on creation in order to set a point
// where it's time to check for ejection conditions.  The actual size may continue
// to rise by the threshold for batch ejection (currently 128)
func WithLimit[K comparable, V any](limit uint) OptionFunc[K, V] {
	return func(be *Backend[K, V]) {
		be.guaranteedCapacity = limit
	}
}

// WithEject can be provided to the backend on creation in order to set a custom
// eject function to pick the entity to be evicted upon overflow, as well as
// hooking into it for additional cleanup work.
func WithEject[K comparable, V any](eject EjectFunc[K, V]) OptionFunc[K, V] {
	return func(be *Backend[K, V]) {
		be.eject = eject
		be.batchEject = nil
	}
}

// WithMutableBackData sets the underlying mutable BackData for the backend.
//
// MutableBackData represents the mutable data structure used by mempool.Backend
// core structure of maintaining data on memory-pools.
func WithMutableBackData[K comparable, V any](mutableBackData mempool.MutableBackData[K, V]) OptionFunc[K, V] {
	return func(be *Backend[K, V]) {
		be.mutableBackData = mutableBackData
	}
}
