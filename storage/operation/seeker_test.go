package operation_test

import (
	"testing"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"

	"github.com/stretchr/testify/require"
)

func TestSeekLE(t *testing.T) {
	dbtest.RunWithStorages(t, func(t *testing.T, r storage.Reader, withWriter dbtest.WithWriter) {

		// Insert the keys into the storage
		codePrefix := byte(1)
		keyParts := []uint64{1, 5, 9}

		require.NoError(t, withWriter(func(writer storage.Writer) error {
			for _, keyPart := range keyParts {
				key := operation.MakePrefix(codePrefix, keyPart)
				value := []byte{0x00} // value are skipped, doesn't matter

				err := operation.Upsert(key, value)(writer)
				if err != nil {
					return err
				}
			}
			return nil
		}))

		t.Run("key below start prefix", func(t *testing.T) {
			seeker := r.NewSeeker()

			key := operation.MakePrefix(codePrefix, uint64(4))
			startPrefix := operation.MakePrefix(codePrefix, uint64(5))

			_, err := seeker.SeekLE(startPrefix, key)
			require.Error(t, err)
		})

		t.Run("seek key inside range", func(t *testing.T) {
			seeker := r.NewSeeker()

			startPrefix := operation.MakePrefix(codePrefix)

			// Seeking 9 and 10 return 9.
			for _, keyPart := range []uint64{9, 10} {
				key := operation.MakePrefix(codePrefix, keyPart)
				expectedKey := operation.MakePrefix(codePrefix, uint64(9))
				foundKey, err := seeker.SeekLE(startPrefix, key)
				require.NoError(t, err)
				require.Equal(t, expectedKey, foundKey)
			}

			// Seeking [5, 8] returns 5.
			for _, keyPart := range []uint64{5, 6, 7, 8} {
				key := operation.MakePrefix(codePrefix, keyPart)
				expectedKey := operation.MakePrefix(codePrefix, uint64(5))
				foundKey, err := seeker.SeekLE(startPrefix, key)
				require.NoError(t, err)
				require.Equal(t, expectedKey, foundKey)
			}

			// Seeking [1, 4] returns 1.
			for _, keyPart := range []uint64{1, 2, 3, 4} {
				key := operation.MakePrefix(codePrefix, keyPart)
				expectedKey := operation.MakePrefix(codePrefix, uint64(1))
				foundKey, err := seeker.SeekLE(startPrefix, key)
				require.NoError(t, err)
				require.Equal(t, expectedKey, foundKey)
			}

			// Seeking 0 returns nil.
			for _, keyPart := range []uint64{0} {
				key := operation.MakePrefix(codePrefix, keyPart)
				foundKey, err := seeker.SeekLE(startPrefix, key)
				require.ErrorIs(t, err, storage.ErrNotFound)
				require.Nil(t, foundKey)
			}
		})

		t.Run("has key below startPrefix", func(t *testing.T) {
			seeker := r.NewSeeker()

			startPrefix := operation.MakePrefix(codePrefix, uint64(6))

			// Key 5 exists, but it is below startPrefix, so nil is returned.
			key := operation.MakePrefix(codePrefix, uint64(6))
			foundKey, err := seeker.SeekLE(startPrefix, key)
			require.ErrorIs(t, err, storage.ErrNotFound)
			require.Nil(t, foundKey)
		})
	})
}
