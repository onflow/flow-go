package mempool

// BackData is a generic key-value storage interface utilized by the mempool.Backend, as the
// core structure of maintaining data on memory-pools.
// NOTE: BackData by default is not expected to provide concurrency-safe operations. As it is just the
// model layer of the mempool, the safety against concurrent operations are guaranteed by the Backend that
// is the control layer.
type BackData[K comparable, V any] interface {
	// Has checks if backdata already contains the entity with the given identifier.
	Has(key K) bool

	// Add adds the value associated with key.
	Add(key K, value V) error

	// Remove removes the value with the given key.
	// Returns the removed value and true if found.
	Remove(key K) (V, bool)

	// Adjust adjusts the value using the provided function if the key is found.
	// It returns the updated value along with a boolean indicating whether an update occurred.
	Adjust(key K, f func(value V) V) (V, bool)

	// AdjustWithInit adjusts the value using the provided function if the key is found.
	// If the key is not found, it initializes the value using the init function and then applies the adjustment.
	//
	// Args:
	//   key: The key for which the value should be adjusted.
	//   adjust: A function that takes the current value and returns the adjusted value.
	//   init: A function that returns an initial value if the key is not present.
	//
	// Returns:
	//   The adjusted value, and a boolean indicating whether the value was adjusted.
	AdjustWithInit(key K, adjust func(value V) V, init func() V) (V, bool)

	// GetWithInit returns the value for the given key.
	// If the key does not exist, it creates a new value using the init function, stores it, and returns it.
	//
	// Args:
	//   key: The key for which the value should be retrieved.
	//   init: A function that returns an initial value if the key is not present.
	//
	// Returns:
	//   The value associated with the key (either existing or newly initialized), and a boolean indicating
	//   whether the value was found (true) or was newly created (false).
	GetWithInit(key K, init func() V) (V, bool)

	// ByID returns the value for the given key.
	ByID(key K) (V, bool)

	// Size returns the number of stored key-value pairs.
	Size() uint

	// All returns all stored key-value pairs as a map.
	All() map[K]V

	// Identifiers returns the list of keys stored in the backdata.
	Identifiers() []K

	// Entities returns the list of stored values.
	Entities() []V

	// Clear removes all key-value pairs from the backdata.
	Clear()
}
