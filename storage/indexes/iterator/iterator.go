package iterator

import (
	"github.com/onflow/flow-go/storage"
)

// Build creates a new index iterator from a storage iterator.
// The returned iterator is a iter.Seq and can be used directly in for loops:
//
//	for entry := range Build(iter, decodeKey, reconstruct) {
//		value, err := entry.Value()
//	}
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
