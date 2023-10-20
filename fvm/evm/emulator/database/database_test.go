package database_test

import (
	"testing"

	"github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/emulator/database"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
)

func TestDatabase(t *testing.T) {

	t.Run("test database", func(t *testing.T) {
		testutils.RunWithTestBackend(t, func(backend types.Backend) {
			testutils.RunWithTestFlowEVMRootAddress(t, backend, func(flowEVMRoot flow.Address) {
				db, err := database.NewDatabase(backend, flowEVMRoot)
				require.NoError(t, err)

				key := []byte("ABC")
				value := []byte{1, 2, 3, 4, 5, 6, 7, 8}
				err = db.Put(key, value)
				require.NoError(t, err)

				err = db.Commit()
				require.NoError(t, err)

				newdb, err := database.NewDatabase(backend, flowEVMRoot)
				require.NoError(t, err)

				retValue, err := newdb.Get(key)
				require.NoError(t, err)

				require.Equal(t, value, retValue)

			})
		})
	})

}
