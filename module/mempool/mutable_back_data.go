package mempool

// MutableBackData extends BackData by allowing modifications to stored data structures.
// Unlike BackData, this interface supports adjusting existing data structures, making it suitable for use cases
// where they do not have a cryptographic hash function.
//
// WARNING: Entities that are cryptographically protected, such as Entity objects tied to signatures or hashes,
// should not be modified. Use BackData instead to prevent unintended mutations.
type MutableBackData[K comparable, V any] interface {
	BackData[K, V]

	// Adjust adjusts the value using the given function if the given key can be found.
	// Returns a bool which indicates whether the value was updated as well as the updated value.
	Adjust(key K, f func(value V) V) (V, bool)

	// AdjustWithInit adjusts the value using the given function if the given key can be found. When the
	// value is not found, it initializes the value using the given init function and then applies the adjust function.
	// Args:
	// - key: The key for which the value should be adjusted.
	// - adjust: the function that adjusts the value.
	// - init: A function that initializes the value if the key is not present.
	// Returns:
	//   - the adjusted value.
	//
	// - a bool which indicates whether the value was adjusted.
	AdjustWithInit(key K, adjust func(value V) V, init func() V) (V, bool)
}
