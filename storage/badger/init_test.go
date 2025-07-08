package badger_test

import (
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestInitPublic(t *testing.T) {
	unittest.RunWithTypedBadgerDB(t, bstorage.InitPublic, func(db *badger.DB) {
		err := operation.EnsurePublicDB(db)
		require.NoError(t, err)
		err = operation.EnsureSecretDB(db)
		require.Error(t, err)
	})
}

func TestInitSecret(t *testing.T) {
	unittest.RunWithTypedBadgerDB(t, bstorage.InitSecret, func(db *badger.DB) {
		err := operation.EnsureSecretDB(db)
		require.NoError(t, err)
		err = operation.EnsurePublicDB(db)
		require.Error(t, err)
	})
}

// opening a database which has previously been opened with encryption enabled,
// using a different encryption key, should fail
func TestEncryptionKeyMismatch(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {

		// open a database with encryption enabled
		key1 := unittest.SeedFixture(32)
		db := unittest.TypedBadgerDB(t, dir, func(options badger.Options) (*badger.DB, error) {
			options = options.WithEncryptionKey(key1)
			return badger.Open(options)
		})
		db.Close()

		// open the same database with a different key
		key2 := unittest.SeedFixture(32)
		opts := badger.
			DefaultOptions(dir).
			WithKeepL0InMemory(true).
			WithEncryptionKey(key2).
			WithLogger(nil)
		_, err := badger.Open(opts)
		// opening the database should return an error
		require.Error(t, err)
	})
}

func TestIsBadgerFolder(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		ok, err := bstorage.IsBadgerFolder(dir)
		require.NoError(t, err)
		require.False(t, ok)

		db := unittest.BadgerDB(t, dir)
		ok, err = bstorage.IsBadgerFolder(dir)
		require.NoError(t, err)
		require.True(t, ok)
		require.NoError(t, db.Close())
	})
}

func TestPebbleIsNotBadgerFolder(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		db, err := pebble.Open(dir, &pebble.Options{})
		require.NoError(t, err)

		ok, err := bstorage.IsBadgerFolder(dir)
		require.NoError(t, err)
		require.False(t, ok)

		require.NoError(t, db.Close())
	})
}
