package operation_test

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestInsertRetrieveDBTypeMarker(t *testing.T) {
	t.Run("should insert and ensure type marker", func(t *testing.T) {
		t.Run("public", func(t *testing.T) {
			unittest.RunWithBadgerDB(t, func(db *badger.DB) {

				// can insert db marker to empty DB
				err := db.Update(operation.InsertPublicDBMarker)
				require.NoError(t, err)
				// can insert db marker twice
				err = db.Update(operation.InsertPublicDBMarker)
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
			unittest.RunWithBadgerDB(t, func(db *badger.DB) {

				// can insert db marker to empty DB
				err := db.Update(operation.InsertSecretDBMarker)
				require.NoError(t, err)
				// can insert db marker twice
				err = db.Update(operation.InsertSecretDBMarker)
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
			unittest.RunWithBadgerDB(t, func(db *badger.DB) {

				// can insert db marker to empty DB
				err := db.Update(operation.InsertPublicDBMarker)
				require.NoError(t, err)
				// inserting a different marker should fail
				err = db.Update(operation.InsertSecretDBMarker)
				require.Error(t, err)
			})
		})
		t.Run("secret", func(t *testing.T) {
			unittest.RunWithBadgerDB(t, func(db *badger.DB) {

				// can insert db marker to empty DB
				err := db.Update(operation.InsertSecretDBMarker)
				require.NoError(t, err)
				// inserting a different marker should fail
				err = db.Update(operation.InsertPublicDBMarker)
				require.Error(t, err)
			})
		})
	})
}
