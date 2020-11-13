// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import "github.com/onflow/flow-go/model/flow"

// OptionFunc is a function that can be provided to the backend on creation in
// order to set a certain custom option.
type OptionFunc func(*Backend)

// OnEjection is a callback which a stdmap.Backend executes on ejecting
// one of its elements. The callbacks are executed from within the thread
// that serves the stdmap.Backend. Implementations should be non-blocking.
type OnEjection func(key flow.Identifier, entity flow.Entity)

// WithLimit can be provided to the backend on creation in order to set a custom
// maximum limit of entities in the memory pool.
func WithLimit(limit uint) OptionFunc {
	return func(be *Backend) {
		be.limit = limit
	}
}

// WithEject can be provided to the backend on creation in order to set a custom
// eject function to pick the entity to be evicted upon overflow, as well as
// hooking into it for additional cleanup work.
func WithEject(eject EjectFunc) OptionFunc {
	return func(be *Backend) {
		be.eject = eject
	}
}

// WithEjectionCallback is an option for the backend at construction time.
// It set a custom ejection callback, which is executed whenever the backend
// ejects an element.
// Only a single ejection callback is supported. Repeating the
// WithEjectionCallback will lead to overriding the function pointer.
// Only the last callback is retained.
func WithEjectionCallback(callback OnEjection) OptionFunc {
	return func(be *Backend) {
		be.ejectionCallback = callback
	}
}
