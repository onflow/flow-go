package mempool

// BackData represents the underlying immutable generic key-value data structure used by mempool.Backend
// as the core structure of maintaining data on memory pools.
//
// This interface provides fundamental operations for storing, retrieving, and removing data structures,
// but it does not support mutating already stored data. If modifications to the stored data is required,
// use [MutableBackData] instead.
//
// NOTE: BackData by default is not expected to provide concurrency-safe operations. As it is just the
// model layer of the mempool, the safety against concurrent operations are guaranteed by the Backend that
// is the control layer.
type BackData[K comparable, V any] interface {
	// Has checks if backdata already stores a value under the given key.
	Has(key K) bool

	// Add attempts to add the given value to the backdata, without overwriting existing data.
	// If a value is already stored under the input key, Add is a no-op and returns false.
	// If no value is stored under the input key, Add adds the value and returns true.
	Add(key K, value V) bool

	// Remove removes the value with the given key.
	// If the key-value pair exists, returns the value and true.
	// Otherwise, returns the zero value for type V and false.
	Remove(key K) (V, bool)

	// GetWithInit returns the given value from the backdata. If the value does not exist, it creates a new value
	// using the factory function and stores it in the backdata.
	// Args:
	// - key: The key for which the value should be retrieved.
	// - init: A function that initializes the value if the key is not present.
	//
	// Returns:
	// - the value.
	// - a bool which indicates whether the value was found (or created).
	GetWithInit(key K, init func() V) (V, bool)

	// Get returns the value for the given key.
	// Returns true if the key-value pair exists, and false otherwise.
	Get(key K) (V, bool)

	// Size returns the number of stored key-value pairs.
	Size() uint

	// All returns all stored key-value pairs as a map.
	All() map[K]V

	// Keys returns an unordered list of keys stored in the backdata.
	Keys() []K

	// Values returns an unordered list of values stored in the backdata.
	Values() []V

	// Clear removes all key-value pairs from the backdata.
	Clear()
}
