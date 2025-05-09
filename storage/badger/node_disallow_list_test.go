package store_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestNodeDisallowedListStoreAndRetrieve(t *testing.T) {
	t.Run("Retrieving non-existing disallowlist should return no error", func(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			nodeDisallowListStore := store.NewNodeDisallowList(db)

			var disallowList map[flow.Identifier]struct{}
			err := nodeDisallowListStore.Retrieve(&disallowList)
			require.NoError(t, err)
			require.Equal(t, 0, len(disallowList))
		})
	})

	t.Run("Storing and read disallowlist", func(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			nodeDisallowListStore := store.NewNodeDisallowList(db)

			disallowList := unittest.IdentifierListFixture(8).Lookup()
			err := nodeDisallowListStore.Store(disallowList)
			require.NoError(t, err)

			var b map[flow.Identifier]struct{}
			err = nodeDisallowListStore.Retrieve(&b)
			require.NoError(t, err)
			require.Equal(t, disallowList, b)
		})
	})

	t.Run("Overwrite disallowlist", func(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			nodeDisallowListStore := store.NewNodeDisallowList(db)

			disallowList1 := unittest.IdentifierListFixture(8).Lookup()
			err := nodeDisallowListStore.Store(disallowList1)
			require.NoError(t, err)

			disallowList2 := unittest.IdentifierListFixture(8).Lookup()
			err = nodeDisallowListStore.Store(disallowList2)
			require.NoError(t, err)

			var b map[flow.Identifier]struct{}
			err = nodeDisallowListStore.Retrieve(&b)
			require.NoError(t, err)
			require.Equal(t, disallowList2, b)
		})
	})

	t.Run("Write & Purge & Write disallowlist", func(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			nodeDisallowListStore := store.NewNodeDisallowList(db)

			disallowList1 := unittest.IdentifierListFixture(8).Lookup()
			err := nodeDisallowListStore.Store(disallowList1)
			require.NoError(t, err)

			err = nodeDisallowListStore.Store(nil)
			require.NoError(t, err)

			var b map[flow.Identifier]struct{}
			err = nodeDisallowListStore.Retrieve(&b)
			require.NoError(t, err)
			require.Equal(t, 0, len(b))

			disallowList2 := unittest.IdentifierListFixture(8).Lookup()
			err = nodeDisallowListStore.Store(disallowList2)
			require.NoError(t, err)

			err = nodeDisallowListStore.Retrieve(&b)
			require.NoError(t, err)
			require.Equal(t, disallowList2, b)
		})
	})

	t.Run("Purge non-existing disallowlis", func(t *testing.T) {
		dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
			nodeDisallowListStore := store.NewNodeDisallowList(db)

			var b map[flow.Identifier]struct{}
			err := nodeDisallowListStore.Retrieve(&b)
			require.NoError(t, err)
			require.Equal(t, 0, len(b))

			err = nodeDisallowListStore.Store(nil)
			require.NoError(t, err)

			err = nodeDisallowListStore.Retrieve(&b)
			require.NoError(t, err)
			require.Equal(t, 0, len(b))
		})
	})
}
