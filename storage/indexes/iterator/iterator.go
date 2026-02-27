package iterator

import (
	"bytes"

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

			cursor, err := decodeKey(key)
			if err != nil {
				if !yield(nil, err) {
					return
				}
				// in practice, the caller is most likely going to break from the loop if there's an error
				// continue here since that is the intuitive behavior.
				continue
			}

			getValue := func(decodedKey C) (*T, error) {
				var result *T
				err := storageItem.Value(func(val []byte) error {
					var err error
					result, err = reconstruct(decodedKey, val)
					return err
				})
				return result, err
			}

			entry := NewEntry(cursor, getValue)
			if !yield(entry, nil) {
				return
			}
		}
	}
}

// BuildPrefixIterator creates a new index iterator from a storage iterator, yielding only the
// first entry per group. Two consecutive keys belong to the same group when keyPrefix
// returns equal byte slices for both. Since the iterator is assumed to be ordered with
// groups contiguous and the desired entry first within each group, this yields exactly
// one entry per group.
func BuildPrefixIterator[T any, C any](
	iter storage.Iterator,
	decodeKey storage.DecodeKeyFunc[C],
	reconstruct storage.ReconstructFunc[T, C],
	keyPrefix func(key []byte) []byte, // must not modify the input slice. returned slice will also not be modifed
) storage.IndexIterator[T, C] {
	return func(yield func(storage.IteratorEntry[T, C], error) bool) {
		defer iter.Close()
		var lastPrefix []byte
		for iter.First(); iter.Valid(); iter.Next() {
			storageItem := iter.IterItem()
			key := storageItem.Key()

			prefix := keyPrefix(key)
			if bytes.Equal(prefix, lastPrefix) {
				continue
			}

			// make a copy since iter will reuse the same memory for the next key
			lastPrefix = append(lastPrefix[:0], prefix...)

			keyCopy := make([]byte, len(key))
			copy(keyCopy, key)

			cursor, err := decodeKey(keyCopy)
			if err != nil {
				if !yield(nil, err) {
					return
				}
				// in practice, the caller is most likely going to break from the loop if there's an error
				// continue here since that is the intuitive behavior.
				continue
			}

			getValue := func(decodedKey C) (*T, error) {
				var result *T
				err := storageItem.Value(func(val []byte) error {
					var err error
					result, err = reconstruct(decodedKey, val)
					return err
				})
				return result, err
			}

			entry := NewEntry(cursor, getValue)
			if !yield(entry, nil) {
				return
			}
		}
	}
}
