package iterator

import "github.com/onflow/flow-go/storage"

// Entry is a single stored entry returned by an index iterator.
type Entry[T any, C any] struct {
	key       []byte
	decodeKey storage.DecodeKeyFunc[C]
	getValue  func(C, *T) error
}

func NewEntry[T any, C any](key []byte, decodeKey storage.DecodeKeyFunc[C], getValue func(C, *T) error) Entry[T, C] {
	return Entry[T, C]{
		key:       key,
		decodeKey: decodeKey,
		getValue:  getValue,
	}
}

// Cursor returns the cursor for the entry, which includes all data included in the storage key.
func (i Entry[T, C]) Cursor() (C, error) {
	return i.decodeKey(i.key)
}

// Value returns the fully reconstructed value for the entry.
func (i Entry[T, C]) Value() (T, error) {
	var v T
	decodedKey, err := i.decodeKey(i.key) // TODO: avoid duplicate decoding
	if err != nil {
		return v, err
	}

	err = i.getValue(decodedKey, &v)
	return v, err
}
