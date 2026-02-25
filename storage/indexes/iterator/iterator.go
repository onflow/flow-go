package iterator

import (
	"github.com/onflow/flow-go/storage"
)

func Build[T any, C any](iter storage.Iterator, decodeKey storage.DecodeKeyFunc[C], reconstruct storage.ReconstructFunc[T, C]) storage.IndexIterator[T, C] {
	return func(yield func(storage.IteratorEntry[T, C]) bool) {
		defer iter.Close()
		for iter.First(); iter.Valid(); iter.Next() {
			storageItem := iter.IterItem()
			key := storageItem.KeyCopy(nil)

			getValue := func(decodedKey C, v *T) error {
				return storageItem.Value(func(val []byte) error {
					return reconstruct(decodedKey, val, v)
				})
			}

			entry := NewEntry(key, decodeKey, getValue)
			if !yield(entry) {
				return
			}
		}
	}
}

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

func (i Entry[T, C]) Cursor() (C, error) {
	return i.decodeKey(i.key)
}

func (i Entry[T, C]) Value() (T, error) {
	var v T
	decodedKey, err := i.decodeKey(i.key) // TODO: avoid duplicate decoding
	if err != nil {
		return v, err
	}

	err = i.getValue(decodedKey, &v)
	return v, err
}
