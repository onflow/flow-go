package operation_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/unittest"
)

// Test_PersistNodeDisallowlist tests the operations:
//   - PersistNodeDisallowList(disallowList map[flow.Identifier]struct{})
//   - RetrieveNodeDisallowLis(disallowList *map[flow.Identifier]struct{})
//   - PurgeNodeDisallowList()
func Test_PersistNodeDisallowlist(t *testing.T) {
	t.Run("Retrieving non-existing disallowlist should return 'storage.ErrNotFound'", func(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			var disallowList map[flow.Identifier]struct{}
			err := operation.RetrieveNodeDisallowList(db.Reader(), &disallowList)
			require.ErrorIs(t, err, storage.ErrNotFound)
		})
	})

	t.Run("Persisting and read disalowlist", func(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			disallowList := unittest.IdentifierListFixture(8).Lookup()
			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.PersistNodeDisallowList(rw.Writer(), disallowList)
			})
			require.NoError(t, err)

			var b map[flow.Identifier]struct{}
			err = operation.RetrieveNodeDisallowList(db.Reader(), &b)
			require.NoError(t, err)
			require.Equal(t, disallowList, b)
		})
	})

	t.Run("Overwrite disallowlist", func(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			disallowList1 := unittest.IdentifierListFixture(8).Lookup()
			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.PersistNodeDisallowList(rw.Writer(), disallowList1)
			})
			require.NoError(t, err)

			disallowList2 := unittest.IdentifierListFixture(8).Lookup()
			err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.PersistNodeDisallowList(rw.Writer(), disallowList2)
			})
			require.NoError(t, err)

			var b map[flow.Identifier]struct{}
			err = operation.RetrieveNodeDisallowList(db.Reader(), &b)
			require.NoError(t, err)
			require.Equal(t, disallowList2, b)
		})
	})

	t.Run("Write & Purge & Write disallowlist", func(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			disallowList1 := unittest.IdentifierListFixture(8).Lookup()
			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.PersistNodeDisallowList(rw.Writer(), disallowList1)
			})
			require.NoError(t, err)

			err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.PurgeNodeDisallowList(rw.Writer())
			})
			require.NoError(t, err)

			var b map[flow.Identifier]struct{}
			err = operation.RetrieveNodeDisallowList(db.Reader(), &b)
			require.ErrorIs(t, err, storage.ErrNotFound)

			disallowList2 := unittest.IdentifierListFixture(8).Lookup()
			err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.PersistNodeDisallowList(rw.Writer(), disallowList2)
			})
			require.NoError(t, err)

			err = operation.RetrieveNodeDisallowList(db.Reader(), &b)
			require.NoError(t, err)
			require.Equal(t, disallowList2, b)
		})
	})

	t.Run("Purge non-existing disallowlist", func(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			var b map[flow.Identifier]struct{}

			err := operation.RetrieveNodeDisallowList(db.Reader(), &b)
			require.ErrorIs(t, err, storage.ErrNotFound)

			err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.PurgeNodeDisallowList(rw.Writer())
			})
			require.NoError(t, err)

			err = operation.RetrieveNodeDisallowList(db.Reader(), &b)
			require.ErrorIs(t, err, storage.ErrNotFound)
		})
	})
}
