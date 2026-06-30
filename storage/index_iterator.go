package storage

import "iter"

// IndexFilter is a function that filters data entries to include in query responses.
// It takes a single entry and returns true if the entry should be included in the response.
type IndexFilter[T any] func(T) bool

// IndexIterator is an iterator over index entries.
// This is intended to be used with the `indexes` package to allow clients to collect filtered results.
type IndexIterator[T any, C any] iter.Seq2[IteratorEntry[T, C], error]

// ReconstructFunc is a function that reconstructs an output value T based on the key and value from storage.
type ReconstructFunc[T any, C any] func(C, []byte) (*T, error)

// DecodeKeyFunc is a function that decodes a storage key into a cursor C.
type DecodeKeyFunc[C any] func([]byte) (C, error)

// IteratorEntry is a single entry returned by an index iterator.
// It provides access to the cursor and value for the entry.
type IteratorEntry[T any, C any] interface {
	// Cursor returns the cursor for the entry, which includes all data included in the storage key.
	Cursor() C

	// Value returns the fully reconstructed value for the entry.
	//
	// Any error indicates the value cannot be reconstructed from the storage value.
	Value() (T, error)
}
