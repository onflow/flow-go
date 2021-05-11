package storage

import "errors"

var (
	// Note: there is another not found error: badger.ErrKeyNotFound. The difference between
	// badger.ErrKeyNotFound and storage.ErrNotFound is that:
	// badger.ErrKeyNotFound is the error returned by the badger API.
	// The storage/badger/operation package defines various badger API calls that might return
	// badger.ErrKeyNotFound. Modules in storage/badger package will use functions from
	// the storage/badger/operation/ package, and converts the badger.ErrKeyNotFound error into
	// storage.ErrNotFound error, so that the storage module user only need to handle the
	// storage.ErrNotFound error.
	ErrNotFound = errors.New("key not found")

	ErrAlreadyExists = errors.New("key already exists")
	ErrDataMismatch  = errors.New("data for key is different")
)
