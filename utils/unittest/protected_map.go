package unittest

import "sync"

// ProtectedMap is a thread-safe map.
type ProtectedMap[K comparable, V any] struct {
	mu sync.RWMutex
	m  map[K]V
}

// NewProtectedMap returns a new ProtectedMap with the given types
func NewProtectedMap[K comparable, V any]() *ProtectedMap[K, V] {
	return &ProtectedMap[K, V]{
		m: make(map[K]V),
	}
}

// Add adds a key-value pair to the map
func (p *ProtectedMap[K, V]) Add(key K, value V) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.m[key] = value
}

// Remove removes a key-value pair from the map
func (p *ProtectedMap[K, V]) Remove(key K) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.m, key)
}

// Has returns true if the map contains the given key
func (p *ProtectedMap[K, V]) Has(key K) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	_, ok := p.m[key]
	return ok
}

// Get returns the value for the given key and a boolean indicating if the key was found
func (p *ProtectedMap[K, V]) Get(key K) (V, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	value, ok := p.m[key]
	return value, ok
}

// ForEach iterates over the map and calls the given function for each key-value pair.
// If the function returns an error, the iteration is stopped and the error is returned.
func (p *ProtectedMap[K, V]) ForEach(fn func(k K, v V) error) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for k, v := range p.m {
		if err := fn(k, v); err != nil {
			return err
		}
	}
	return nil
}

// Size returns the size of the map.
func (p *ProtectedMap[K, V]) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.m)
}
