package backdata

// MapBackData implements a map-based generic memory BackData backed by a Go map.
// Note that this implementation is NOT thread-safe, and the higher-level Backend is responsible for concurrency management.
type MapBackData[K comparable, V any] struct {
	// NOTE: as a BackData implementation, MapBackData must be non-blocking.
	// Concurrency management is done by overlay Backend.
	dataMap map[K]V
}

func NewMapBackData[K comparable, V any]() *MapBackData[K, V] {
	bd := &MapBackData[K, V]{
		dataMap: make(map[K]V),
	}
	return bd
}

// Has checks if backdata already contains the value with the given key.
func (b *MapBackData[K, V]) Has(key K) bool {
	_, exists := b.dataMap[key]
	return exists
}

// Add adds the given value to the backdata.
func (b *MapBackData[K, V]) Add(key K, value V) bool {
	_, exists := b.dataMap[key]
	if exists {
		return false
	}
	b.dataMap[key] = value
	return true
}

// Remove removes the value with the given key.
func (b *MapBackData[K, V]) Remove(key K) (V, bool) {
	value, exists := b.dataMap[key]
	if !exists {
		var zero V
		return zero, false
	}
	delete(b.dataMap, key)
	return value, true
}

// Adjust adjusts the value using the given function if the given key can be found.
// It returns the updated value along with a boolean indicating whether an update occurred.
func (b *MapBackData[K, V]) Adjust(key K, f func(V) (K, V)) (V, bool) {
	value, ok := b.dataMap[key]
	if !ok {
		var zero V
		return zero, false
	}
	newKey, newValue := f(value)

	delete(b.dataMap, key)
	b.dataMap[newKey] = newValue
	return newValue, true
}

// AdjustWithInit adjusts the value using the provided function if the key is found.
// If the key is not found, it initializes the value using the given init function and then applies the adjustment.
//
// Args:
// - key: The key for which the value should be adjusted.
// - adjust: the function that adjusts the value.
// - init: A function that initializes the value if the key is not present.
//
// Returns:
// - the adjusted value.
//
// - a bool which indicates whether the value was adjusted.
func (b *MapBackData[K, V]) AdjustWithInit(key K, adjust func(V) (K, V), init func() V) (V, bool) {
	if b.Has(key) {
		return b.Adjust(key, adjust)
	}
	b.Add(key, init())
	return b.Adjust(key, adjust)
}

// GetWithInit returns the value for the given key.
// If the key does not exist, it creates a new value using the init function, stores it, and returns it.
//
// Args:
// - key: The key for which the value should be retrieved.
// - init: A function that initializes the value if the key is not present.
//
// Returns:
//   - the value.
//   - a bool which indicates whether the value was found (or created).
func (b *MapBackData[K, V]) GetWithInit(key K, init func() V) (V, bool) {
	if b.Has(key) {
		return b.ByID(key)
	}
	b.Add(key, init())
	return b.ByID(key)
}

// ByID returns the value for the given key.
func (b *MapBackData[K, V]) ByID(key K) (V, bool) {
	value, exists := b.dataMap[key]
	if !exists {
		var zero V
		return zero, false
	}
	return value, true
}

// Size returns the number of stored key-value pairs.
func (b *MapBackData[K, V]) Size() uint {
	return uint(len(b.dataMap))
}

// All returns all stored key-value pairs as a map.
func (b *MapBackData[K, V]) All() map[K]V {
	values := make(map[K]V)
	for key, value := range b.dataMap {
		values[key] = value
	}
	return values
}

// Identifiers returns the list of keys of values stored in the backdata.
func (b *MapBackData[K, V]) Identifiers() []K {
	keys := make([]K, len(b.dataMap))
	i := 0
	for key := range b.dataMap {
		keys[i] = key
		i++
	}
	return keys
}

// Entities returns the list of values stored in the backdata.
func (b *MapBackData[K, V]) Entities() []V {
	values := make([]V, len(b.dataMap))
	i := 0
	for _, value := range b.dataMap {
		values[i] = value
		i++
	}
	return values
}

// Clear removes all values from the backdata.
func (b *MapBackData[K, V]) Clear() {
	b.dataMap = make(map[K]V)
}
