package database_test

import (
	"fmt"
	"testing"

	"github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/emulator/database"
	"github.com/onflow/flow-go/fvm/evm/testutils"
)

func TestDatabase(t *testing.T) {

	t.Run("test database", func(t *testing.T) {
		testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
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

				fmt.Println(backend.TotalStorageSize())

				err = newdb.Delete(key)
				require.NoError(t, err)

				fmt.Println(backend.TotalStorageSize())
				t.Fatal("XXX")

			})
		})
	})

}
