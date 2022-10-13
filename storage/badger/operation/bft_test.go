package operation

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

// Test_PersistBlocklist tests the operations:
//   - PersistBlocklist(blocklist map[flow.Identifier]struct{})
//   - RetrieveBlocklist(blocklist *map[flow.Identifier]struct{})
//   - PurgeBlocklist()
func Test_PersistBlocklist(t *testing.T) {
	t.Run("Retrieving non-existing blocklist should return 'storage.ErrNotFound'", func(t *testing.T) {
		unittest.RunWithBadgerDB(t, func(db *badger.DB) {
			var blocklist map[flow.Identifier]struct{}
			err := db.View(RetrieveBlocklist(&blocklist))
			require.ErrorIs(t, err, storage.ErrNotFound)

		})
	})

	t.Run("Persisting and read blocklist", func(t *testing.T) {
		unittest.RunWithBadgerDB(t, func(db *badger.DB) {
			blocklist := unittest.IdentifierListFixture(8).Lookup()
			err := db.Update(PersistBlocklist(blocklist))
			require.NoError(t, err)

			var b map[flow.Identifier]struct{}
			err = db.View(RetrieveBlocklist(&b))
			require.NoError(t, err)
			require.Equal(t, blocklist, b)
		})
	})

	t.Run("Overwrite blocklist", func(t *testing.T) {
		unittest.RunWithBadgerDB(t, func(db *badger.DB) {
			blocklist1 := unittest.IdentifierListFixture(8).Lookup()
			err := db.Update(PersistBlocklist(blocklist1))
			require.NoError(t, err)

			blocklist2 := unittest.IdentifierListFixture(8).Lookup()
			err = db.Update(PersistBlocklist(blocklist2))
			require.NoError(t, err)

			var b map[flow.Identifier]struct{}
			err = db.View(RetrieveBlocklist(&b))
			require.NoError(t, err)
			require.Equal(t, blocklist2, b)
		})
	})

	t.Run("Write & Purge & Write blocklist", func(t *testing.T) {
		unittest.RunWithBadgerDB(t, func(db *badger.DB) {
			blocklist1 := unittest.IdentifierListFixture(8).Lookup()
			err := db.Update(PersistBlocklist(blocklist1))
			require.NoError(t, err)

			err = db.Update(PurgeBlocklist())
			require.NoError(t, err)

			var b map[flow.Identifier]struct{}
			err = db.View(RetrieveBlocklist(&b))
			require.ErrorIs(t, err, storage.ErrNotFound)

			blocklist2 := unittest.IdentifierListFixture(8).Lookup()
			err = db.Update(PersistBlocklist(blocklist2))
			require.NoError(t, err)

			err = db.View(RetrieveBlocklist(&b))
			require.NoError(t, err)
			require.Equal(t, blocklist2, b)
		})
	})

	t.Run("Purge non-existing blocklist", func(t *testing.T) {
		unittest.RunWithBadgerDB(t, func(db *badger.DB) {
			var b map[flow.Identifier]struct{}

			err := db.View(RetrieveBlocklist(&b))
			require.ErrorIs(t, err, storage.ErrNotFound)

			err = db.Update(PurgeBlocklist())
			require.NoError(t, err)

			err = db.View(RetrieveBlocklist(&b))
			require.ErrorIs(t, err, storage.ErrNotFound)
		})
	})
}
