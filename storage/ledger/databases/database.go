package databases

import "errors"

var ErrNotFound = errors.New("trie kv: not found")

type DAL interface {
	// UpdateTrieDB updates the TrieDB according to the batcher.
	UpdateTrieDB(batcher Batcher) error

	// UpdateKVDB updates the key-value database according to the key-value pairs.
	UpdateKVDB(keys [][]byte, values [][]byte) error

	// GetTrieDB gets the key from the trie database.
	GetTrieDB(key []byte) ([]byte, error)

	// GetKVDB gets the key from the key-value database.
	GetKVDB(key []byte) ([]byte, error)

	// NewBatcher creates a new batcher to be used while updating.
	NewBatcher() Batcher

	// CopyTo copies the contents of this database to the provided database.
	CopyTo(dest DAL) error

	// PruneDB removes all values from this database that also exist in the provided database.
	PruneDB(next DAL) error

	// SafeClose attempts to safely close the trie and key-value databases.
	SafeClose() (error, error)
}

type Batcher interface {
	Put(key []byte, value []byte)
}
