package storage

import (
	"errors"
)

var (
	// ErrNotFound is returned when a retrieved key does not exist in the database.
	// Note: there is another not found error: badger.ErrKeyNotFound. The difference between
	// badger.ErrKeyNotFound and storage.ErrNotFound is that:
	// badger.ErrKeyNotFound is the error returned by the badger API.
	// Modules in storage/badger and storage/badger/operation package both
	// return storage.ErrNotFound for not found error
	ErrNotFound = errors.New("key not found")

	// ErrAlreadyExists is returned when an insert attempts to set the value
	// for a key that already exists. Inserts may only occur once per key,
	// updates may overwrite an existing key without returning an error.
	ErrAlreadyExists = errors.New("key already exists")

	// ErrDataMismatch is returned when a repeatable insert operation attempts
	// to insert a different value for the same key.
	ErrDataMismatch = errors.New("data for key is different")

	// ErrHeightNotIndexed is returned when data that is indexed sequentially is queried by a given block height
	// and that data is unavailable.
	ErrHeightNotIndexed = errors.New("data for block height not available")

	// ErrNotBootstrapped is returned when the database has not been bootstrapped.
	ErrNotBootstrapped = errors.New("pebble database not bootstrapped")
)
