package iterator

import (
	"github.com/onflow/flow-go/storage"
)

// Build creates a new index iterator from a storage iterator.
// The returned iterator is a iter.Seq and can be used directly in for loops:
//
//	for entry, err := range Build(iter, decodeKey, reconstruct) {
//		if err != nil {
//			return err
//		}
//		value, err := entry.Value()
//		if err != nil {
//			return err
//		}
//		// use value
//	}
func Build[T any, C any](iter storage.Iterator, decodeKey storage.DecodeKeyFunc[C], reconstruct storage.ReconstructFunc[T, C]) storage.IndexIterator[T, C] {
	return func(yield func(storage.IteratorEntry[T, C], error) bool) {
		defer iter.Close()
		for iter.First(); iter.Valid(); iter.Next() {
			storageItem := iter.IterItem()
			key := storageItem.KeyCopy(nil)

			getValue := func(decodedKey C) (*T, error) {
				var result *T
				err := storageItem.Value(func(val []byte) error {
					var err error
					result, err = reconstruct(decodedKey, val)
					return err
				})
				return result, err
			}

			cursor, err := decodeKey(key)
			if err != nil {
				if !yield(nil, err) {
					return
				}
			} else {
				entry := NewEntry(cursor, getValue)
				if !yield(entry, nil) {
					return
				}
			}
		}
	}
}
