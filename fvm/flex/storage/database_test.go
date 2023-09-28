package storage_test

import (
	"testing"

	"github.com/onflow/flow-go/fvm/flex/models"
	"github.com/onflow/flow-go/fvm/flex/storage"
	"github.com/onflow/flow-go/fvm/flex/utils"
	"github.com/stretchr/testify/require"
)

func TestDatabase(t *testing.T) {
	t.Run("test storage", func(t *testing.T) {
		utils.RunWithTestBackend(t, func(backend models.Backend) {
			db := storage.NewDatabase(backend)

			key := []byte("ABC")
			value := []byte{1, 2, 3, 4, 5, 6, 7, 8}
			err := db.Put(key, value)
			require.NoError(t, err)

			err = db.Commit()
			require.NoError(t, err)

			newdb := storage.NewDatabase(backend)
			retValue, err := newdb.Get(key)
			require.NoError(t, err)

			require.Equal(t, value, retValue)

		})
	})

}
