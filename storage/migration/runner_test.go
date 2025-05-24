package migration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"
)

func TestRunMigration(t *testing.T) {
	// Setup temporary directories
	tmpDir := t.TempDir()
	badgerDir := filepath.Join(tmpDir, "badger")
	pebbleDir := filepath.Join(tmpDir, "pebble")

	// Create and open BadgerDB with test data
	opts := badger.DefaultOptions(badgerDir).WithLogger(nil)
	badgerDB, err := badger.Open(opts)
	require.NoError(t, err)

	testData := map[string]string{
		"\x01\x02foo": "bar",
		"\x01\x02baz": "qux",
		"\x02\xffzip": "zap",
		"\xff\xffzz":  "last",
	}
	err = badgerDB.Update(func(txn *badger.Txn) error {
		for k, v := range testData {
			err := txn.Set([]byte(k), []byte(v))
			require.NoError(t, err)
		}
		return nil
	})
	require.NoError(t, err)
	require.NoError(t, badgerDB.Close()) // Close so MigrateAndValidate can reopen it

	// Define migration config
	cfg := MigrationConfig{
		BatchByteSize:          1024,
		ReaderWorkerCount:      2,
		WriterWorkerCount:      2,
		ReaderShardPrefixBytes: 2,
	}

	// Run migration
	err = RunMigration(badgerDir, pebbleDir, cfg)
	require.NoError(t, err)

	// Check marker files
	startedPath := filepath.Join(pebbleDir, "MIGRATION_STARTED")
	completedPath := filepath.Join(pebbleDir, "MIGRATION_COMPLETED")

	startedContent, err := os.ReadFile(startedPath)
	require.NoError(t, err)
	require.Contains(t, string(startedContent), "migration started")

	completedContent, err := os.ReadFile(completedPath)
	require.NoError(t, err)
	require.Contains(t, string(completedContent), "migration completed")

	// Open PebbleDB to confirm migrated values
	pebbleDB, err := pebble.Open(pebbleDir, &pebble.Options{})
	require.NoError(t, err)
	defer pebbleDB.Close()

	for k, expected := range testData {
		val, closer, err := pebbleDB.Get([]byte(k))
		require.NoError(t, err)
		require.Equal(t, expected, string(val))
		require.NoError(t, closer.Close())
	}
}
