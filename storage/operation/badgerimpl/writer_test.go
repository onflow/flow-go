package badgerimpl_test

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

// TestBadgerSetArgumentsNotSafeToModify verifies that WriteBatch.Set()
// arguments are not safe to be modified before batch is committed.
func TestBadgerSetArgumentsNotSafeToModify(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		b := db.NewWriteBatch()

		k := []byte{0x01}
		v := []byte{0x01, 0x02, 0x03}

		// Insert k and v to batch.
		err := b.Set(k, v)
		require.NoError(t, err)

		// Modify k and v before commit.
		k[0] = 0x02
		v[0] = 0x04

		// Commit pending writes.
		err = b.Flush()
		require.NoError(t, err)

		tx := db.NewTransaction(false)

		// Retrieve value by original key returns ErrKeyNotFound error.
		_, err = tx.Get([]byte{0x01})
		require.ErrorIs(t, err, badger.ErrKeyNotFound)

		// Retrieve value by modified key returns modified value.
		item, err := tx.Get([]byte{0x02})
		require.NoError(t, err, nil)

		retrievedValue, err := item.ValueCopy(nil)
		require.NoError(t, err, nil)
		require.Equal(t, v, retrievedValue)
	})
}
