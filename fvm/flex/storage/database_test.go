package storage_test

import (
	"testing"

	"github.com/onflow/flow-go/fvm/flex/models"
	"github.com/onflow/flow-go/fvm/flex/storage"
	"github.com/onflow/flow-go/fvm/flex/testutils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/require"
)

func TestDatabase(t *testing.T) {

	t.Run("test storage", func(t *testing.T) {
		testutils.RunWithTestBackend(t, func(backend models.Backend) {
			testutils.RunWithTestFlexRoot(t, backend, func(flexRoot flow.Address) {
				db := storage.NewDatabase(backend, flexRoot)

				key := []byte("ABC")
				value := []byte{1, 2, 3, 4, 5, 6, 7, 8}
				err := db.Put(key, value)
				require.NoError(t, err)

				err = db.Commit()
				require.NoError(t, err)

				newdb := storage.NewDatabase(backend, flexRoot)
				retValue, err := newdb.Get(key)
				require.NoError(t, err)

				require.Equal(t, value, retValue)

			})
		})
	})

}
