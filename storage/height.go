package storage

// HeightIndex defines methods for indexing height.
// This interface should normally be composed with some other resource we want to index by height.
type HeightIndex interface {
	// LatestHeight returns the latest indexed height.
	LatestHeight() (uint64, error)
	// FirstHeight at which we started to index. Returns the first indexed height found in the store.
	FirstHeight() (uint64, error)
	// SetLatestHeight updates the latest height.
	// The provided height should either be one higher than the current height or the same to ensure idempotency.
	// If the height is not within those bounds it will panic!
	// An error might get returned if there are problems with persisting the height.
	SetLatestHeight(height uint64) error
}
