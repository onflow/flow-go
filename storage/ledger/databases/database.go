package databases

type DAL interface {
	// PutIntoBatcher puts key-value pairs into a batcher.
	PutIntoBatcher(key []byte, value []byte)

	// NewBatch creates a new batcher and cleans the old one.
	NewBatch()

	// UpdateTrieDB updates the TrieDB according to the batcher.
	UpdateTrieDB() error

	// UpdateKVDB updates the key-value database according to the KV pair.
	UpdateKVDB(keys [][]byte, values [][]byte) error

	// GetTrieDB gets the key from the TrieDB.
	GetTrieDB(key []byte) ([]byte, error)

	// GetKVDB gets the key from the KVDB.
	GetKVDB(key []byte) ([]byte, error)

	// SafeClose attempts to safely close the databases.
	SafeClose() (error, error)
}
