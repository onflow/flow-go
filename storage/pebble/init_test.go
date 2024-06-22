package pebble_test

import (
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/require"

	bstorage "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/storage/pebble/operation"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestInitPublic(t *testing.T) {
	unittest.RunWithTypedPebbleDB(t, bstorage.InitPublic, func(db *pebble.DB) {
		err := operation.EnsurePublicDB(db)
		require.NoError(t, err)
		err = operation.EnsureSecretDB(db)
		require.Error(t, err)
	})
}

func TestInitSecret(t *testing.T) {
	unittest.RunWithTypedPebbleDB(t, bstorage.InitSecret, func(db *pebble.DB) {
		err := operation.EnsureSecretDB(db)
		require.NoError(t, err)
		err = operation.EnsurePublicDB(db)
		require.Error(t, err)
	})
}

// opening a database which has previously been opened with encryption enabled,
// using a different encryption key, should fail
func TestEncryptionKeyMismatch(t *testing.T) {
	t.Fail()
	// unittest.RunWithTempDir(t, func(dir string) {

	// open a database with encryption enabled
	// 	key1 := unittest.SeedFixture(32)
	// 	db := unittest.TypedPebbleDB(t, dir, func(options pebble.Options) (*pebble.DB, error) {
	// 		options = options.WithEncryptionKey(key1)
	// 		return pebble.Open(options)
	// 	})
	// 	db.Close()
	//
	// 	// open the same database with a different key
	// 	key2 := unittest.SeedFixture(32)
	// 	opts := pebble.
	// 		DefaultOptions(dir).
	// 		WithKeepL0InMemory(true).
	// 		WithEncryptionKey(key2).
	// 		WithLogger(nil)
	// 	_, err := pebble.Open(opts)
	// 	// opening the database should return an error
	// 	require.Error(t, err)
	// })
}
