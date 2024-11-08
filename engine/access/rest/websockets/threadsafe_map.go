package websockets

import (
	"sync"
)

// ThreadSafeMap is a thread-safe map with read-write locking.
type ThreadSafeMap[K comparable, V any] struct {
	mu sync.RWMutex
	m  map[K]V
}

// NewThreadSafeMap initializes a new ThreadSafeMap.
func NewThreadSafeMap[K comparable, V any]() *ThreadSafeMap[K, V] {
	return &ThreadSafeMap[K, V]{
		m: make(map[K]V),
	}
}

// Get retrieves a value for a key, returning the value and a boolean indicating if the key exists.
func (s *ThreadSafeMap[K, V]) Get(key K) (V, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.m[key]
	return value, ok
}

// Insert inserts or updates a value for a key.
func (s *ThreadSafeMap[K, V]) Insert(key K, value V) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = value
}

// Remove removes a key and its value from the map.
func (s *ThreadSafeMap[K, V]) Remove(key K) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, key)
}

// Exists checks if a key exists in the map.
func (s *ThreadSafeMap[K, V]) Exists(key K) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.m[key]
	return ok
}

// Len returns the number of elements in the map.
func (s *ThreadSafeMap[K, V]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.m)
}
