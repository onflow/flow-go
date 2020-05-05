package inmemdb_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/storage/ledger/databases/inmemdb"
	"github.com/dapperlabs/flow-go/storage/ledger/utils"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestPersistance(t *testing.T) {
	// TODO test batcher update to tdb
	unittest.RunWithTempDir(t, func(dbDir string) {
		db, err := inmemdb.NewInMemDB(dbDir)
		require.NoError(t, err, "error creating db")

		keys := utils.GetRandomKeysRandN(20, 32)
		values := utils.GetRandomValues(len(keys), 64)
		db.UpdateKVDB(keys, values)

		value, err := db.GetKVDB(keys[0])
		require.NoError(t, err, "error returning value")
		require.Equal(t, value, values[0])

		err = db.Persist()
		require.NoError(t, err, "error persisting to disk")

		db = nil
		newDB, err := inmemdb.NewInMemDB(dbDir)
		require.NoError(t, err, "error creating new db")

		value, err = newDB.GetKVDB(keys[0])
		require.NoError(t, err, "error returning value from new db")
		require.Equal(t, value, values[0])

	})
}
