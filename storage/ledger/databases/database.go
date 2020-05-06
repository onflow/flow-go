package databases

import "errors"

var ErrNotFound = errors.New("trie kv: not found")

type DAL interface {
	// PutIntoBatcher puts key-value pairs into a batcher.
	//PutIntoBatcher(key []byte, value []byte)
	//

	// NewBatcher creates a new batcher to be used while updating
	NewBatcher() Batcher

	// UpdateTrieDB updates the TrieDB according to the batcher.
	UpdateTrieDB(batcher Batcher) error

	// UpdateKVDB updates the key-value database according to the KV pair.
	UpdateKVDB(keys [][]byte, values [][]byte) error

	// GetTrieDB gets the key from the TrieDB.
	GetTrieDB(key []byte) ([]byte, error)

	// GetKVDB gets the key from the KVDB.
	GetKVDB(key []byte) ([]byte, error)

	CopyTo(db DAL) error

	// PruneDB removes all values from this database that also exist in the provided database.
	PruneDB(next DAL) error

	// SafeClose attempts to safely close the databases.
	SafeClose() (error, error)
}
type Batcher interface {
	Put(key []byte, value []byte)
	Merge(Batcher)
}
