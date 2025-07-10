// Package synchronized provides a generic thread-safe wrapper for accessing data
// using sync.RWMutex with scoped locking.
//
// The main problem it solves is: when you define a struct like:
//
//	struct {
//	    mu   sync.Mutex
//	    data int
//	}
//
// It’s easy to accidentally access or mutate `data` without acquiring the lock.
// This leads to subtle and dangerous data races.
//
// The Synchronized[T] type enforces safe access to the underlying data by requiring
// access to happen through closures that manage locking automatically.
//
// It eliminates the risk of forgetting to lock before accessing or modifying the data,
// by centralizing all access inside closure-based methods.
package synchronized

import (
	"sync"
)

// Synchronized wraps a value of any type T and synchronizes access to it
// using an internal sync.RWMutex.
type Synchronized[T any] struct {
	data T
	mu   sync.RWMutex //TODO: should we allow user to specify mutex type? Because we can make it generic
}

// New initializes a new Synchronized wrapper with the given initial value.
func New[T any](value T) Synchronized[T] {
	return Synchronized[T]{data: value}
}

// WithReadLock provides read-only access to a copy of the value.
// The caller receives a snapshot of the data, safe for concurrent use.
// This is ideal for small or copyable types where mutation isn't required.
func (s *Synchronized[T]) WithReadLock(f func(data T)) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f(s.data)
}

// WithReadPointer provides read-only access to a pointer to the underlying data.
// The caller must not mutate the data. Useful for large or complex structures
// where copying is expensive.
//
// ⚠️ Use with caution: mutating the data while holding only a read lock
// will lead to data races and undefined behavior.
func (s *Synchronized[T]) WithReadPointer(f func(data *T)) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f(&s.data)
}

// WithWriteLock provides exclusive access to the underlying data via a pointer.
// The caller may freely mutate the data inside the provided closure.
func (s *Synchronized[T]) WithWriteLock(f func(data *T)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f(&s.data)
}
