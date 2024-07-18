package operation_test

import (
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage/pebble/operation"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestInsertRetrieveDBTypeMarker(t *testing.T) {
	t.Run("should insert and ensure type marker", func(t *testing.T) {
		t.Run("public", func(t *testing.T) {
			unittest.RunWithPebbleDB(t, func(db *pebble.DB) {

				// can insert db marker to empty DB
				err := operation.InsertPublicDBMarker(db)
				require.NoError(t, err)
				// can insert db marker twice
				err = operation.InsertPublicDBMarker(db)
				require.NoError(t, err)
				// ensure correct db type succeeds
				err = operation.EnsurePublicDB(db)
				require.NoError(t, err)
				// ensure other db type fails
				err = operation.EnsureSecretDB(db)
				require.Error(t, err)
			})
		})

		t.Run("secret", func(t *testing.T) {
			unittest.RunWithPebbleDB(t, func(db *pebble.DB) {

				// can insert db marker to empty DB
				err := operation.InsertSecretDBMarker(db)
				require.NoError(t, err)
				// can insert db marker twice
				err = operation.InsertSecretDBMarker(db)
				require.NoError(t, err)
				// ensure correct db type succeeds
				err = operation.EnsureSecretDB(db)
				require.NoError(t, err)
				// ensure other db type fails
				err = operation.EnsurePublicDB(db)
				require.Error(t, err)
			})
		})
	})

	t.Run("should fail to insert different db marker to non-empty db", func(t *testing.T) {
		t.Run("public", func(t *testing.T) {
			unittest.RunWithPebbleDB(t, func(db *pebble.DB) {

				// can insert db marker to empty DB
				err := operation.InsertPublicDBMarker(db)
				require.NoError(t, err)
				// inserting a different marker should fail
				err = operation.InsertSecretDBMarker(db)
				require.Error(t, err)
			})
		})
		t.Run("secret", func(t *testing.T) {
			unittest.RunWithPebbleDB(t, func(db *pebble.DB) {

				// can insert db marker to empty DB
				err := operation.InsertSecretDBMarker(db)
				require.NoError(t, err)
				// inserting a different marker should fail
				err = operation.InsertPublicDBMarker(db)
				require.Error(t, err)
			})
		})
	})
}
