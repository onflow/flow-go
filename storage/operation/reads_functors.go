package operation

import "github.com/onflow/flow-go/storage"

// Leo: This package includes deprecated functions that wraps the operation of reading from the database.
// They are needed because the original badger implementation is also implemented in the same wrapped function manner,
// since badger requires reads to be done in a transaction, which is stateful.
// Using these deprecated functions could minimize the changes during refactor and easier to review the changes.
// The simplified implementation of the functions are in the reads.go file, which are encouraged to be used instead.

func Iterate(startPrefix []byte, endPrefix []byte, check func(key []byte) error) func(storage.Reader) error {
	return func(r storage.Reader) error {
		return IterateKeysByPrefixRange(r, startPrefix, endPrefix, check)
	}
}

func Traverse(prefix []byte, iterFunc IterationFunc, opt storage.IteratorOption) func(storage.Reader) error {
	return func(r storage.Reader) error {
		return TraverseByPrefix(r, prefix, iterFunc, opt)
	}
}

func Retrieve(key []byte, entity any) func(storage.Reader) error {
	return func(r storage.Reader) error {
		return RetrieveByKey(r, key, entity)
	}
}

func Exists(key []byte, keyExists *bool) func(storage.Reader) error {
	return func(r storage.Reader) error {
		exists, err := KeyExists(r, key)
		if err != nil {
			return err
		}
		*keyExists = exists
		return nil
	}
}

func FindHighestAtOrBelow(prefix []byte, height uint64, entity any) func(storage.Reader) error {
	return func(r storage.Reader) error {
		return FindHighestAtOrBelowByPrefix(r, prefix, height, entity)
	}
}
