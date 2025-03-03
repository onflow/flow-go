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
	// Returns:
	//    - value, true if the value with the given key was found. The returned value is the version after the update is applied.
	//    - nil, false if no value with the given key was found
	Adjust(key K, f func(value V) (K, V)) (V, bool)

	// AdjustWithInit adjusts the value using the given function if the given key can be found. When the
	// value is not found, it initializes the value using the given init function and then applies the adjust function.
	// Args:
	// - key: the identifier of the value to adjust.
	// - adjust: the function that adjusts the value.
	// - init: the function that initializes the value when it is not found.
	// Returns:
	//   - the adjusted value.
	//   - a bool which indicates whether the value was either added or adjusted.
	AdjustWithInit(key K, adjust func(value V) (K, V), init func() V) (V, bool)
}
