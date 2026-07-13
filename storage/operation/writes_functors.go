package operation

import "github.com/onflow/flow-go/storage"

// Leo: This package includes deprecated functions that wraps the operation of writing to the database.
// They are needed because the original badger implementation is also implemented in the same wrapped function manner,
// since badger requires writes to be done in a transaction, which is stateful.
// Using these deprecated functions could minimize the changes during refactor and easier to review the changes.
// The simplified implementation of the functions are in the writes.go file, which are encouraged to be used instead.

func Upsert(key []byte, val any) func(storage.Writer) error {
	return func(w storage.Writer) error {
		return UpsertByKey(w, key, val)
	}
}

func Remove(key []byte) func(storage.Writer) error {
	return func(w storage.Writer) error {
		return RemoveByKey(w, key)
	}
}

func RemoveByPrefix(reader storage.Reader, key []byte) func(storage.Writer) error {
	return func(w storage.Writer) error {
		return RemoveByKeyPrefix(reader, w, key)
	}
}

func RemoveByRange(reader storage.Reader, startPrefix []byte, endPrefix []byte) func(storage.Writer) error {
	return func(w storage.Writer) error {
		return RemoveByKeyRange(reader, w, startPrefix, endPrefix)
	}
}
