package mempool

// Mempool is a generic interface for concurrency-safe memory pool.
type Mempool[K comparable, V any] interface {
	// Has checks if a value is stored under the given key.
	Has(K) bool
	// Get returns the value for the given key.
	// Returns true if the key-value pair exists, and false otherwise.
	Get(K) (V, bool)
	// Add attempts to add the given value, without overwriting existing data.
	// If a value is already stored under the input key, Add is a no-op and returns false.
	// If no value is stored under the input key, Add adds the value and returns true.
	Add(K, V) bool
	// Remove removes the value with the given key.
	// If the key-value pair exists, returns the value and true.
	// Otherwise, returns the zero value for type V and false.
	Remove(K) bool
	// Adjust will adjust the value item using the given function if the given key can be found.
	// Returns:
	//   - value, true if the value with the given key was found. The returned value is the version after the update is applied.
	//   - nil, false if no value with the given key was found
	Adjust(key K, f func(V) V) (V, bool)
	// Size will return the size of the mempool.
	Size() uint
	// All returns all stored key-value pairs as a map from the mempool.
	All() map[K]V
	// Clear removes all key-value pairs from the mempool.
	Clear()
}
