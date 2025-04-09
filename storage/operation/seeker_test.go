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
			endPrefix := operation.MakePrefix(codePrefix, uint64(10))

			_, _, err := seeker.SeekLE(startPrefix, endPrefix, key)
			require.Error(t, err)
		})

		t.Run("key above end prefix", func(t *testing.T) {
			seeker := r.NewSeeker()

			startPrefix := operation.MakePrefix(codePrefix, uint64(0))
			endPrefix := operation.MakePrefix(codePrefix, uint64(5))
			key := operation.MakePrefix(codePrefix, uint64(10))

			_, _, err := seeker.SeekLE(startPrefix, endPrefix, key)
			require.Error(t, err)
		})

		t.Run("seek key inside range", func(t *testing.T) {
			seeker := r.NewSeeker()

			// Seek range is [0, 10].
			startPrefix := operation.MakePrefix(codePrefix)
			endPrefix := operation.MakePrefix(codePrefix, uint64(10))

			// Seeking [9, 10] in range of [0, 10] returns 9.
			for _, keyPart := range []uint64{9, 10} {
				key := operation.MakePrefix(codePrefix, keyPart)
				expectedKey := operation.MakePrefix(codePrefix, uint64(9))
				foundKey, found, err := seeker.SeekLE(startPrefix, endPrefix, key)
				require.NoError(t, err)
				require.True(t, found)
				require.Equal(t, expectedKey, foundKey)
			}

			// Seeking [5, 8] in range of [0, 10] returns 5.
			for _, keyPart := range []uint64{5, 6, 7, 8} {
				key := operation.MakePrefix(codePrefix, keyPart)
				expectedKey := operation.MakePrefix(codePrefix, uint64(5))
				foundKey, found, err := seeker.SeekLE(startPrefix, endPrefix, key)
				require.NoError(t, err)
				require.True(t, found)
				require.Equal(t, expectedKey, foundKey)
			}

			// Seeking [1, 4] in range of [0, 10] returns 1.
			for _, keyPart := range []uint64{1, 2, 3, 4} {
				key := operation.MakePrefix(codePrefix, keyPart)
				expectedKey := operation.MakePrefix(codePrefix, uint64(1))
				foundKey, found, err := seeker.SeekLE(startPrefix, endPrefix, key)
				require.NoError(t, err)
				require.True(t, found)
				require.Equal(t, expectedKey, foundKey)
			}

			// Seeking [0] in range of [0, 10] returns nil.
			for _, keyPart := range []uint64{0} {
				key := operation.MakePrefix(codePrefix, keyPart)
				foundKey, found, err := seeker.SeekLE(startPrefix, endPrefix, key)
				require.NoError(t, err)
				require.False(t, found)
				require.Nil(t, foundKey)
			}
		})

		t.Run("has key below startPrefix", func(t *testing.T) {
			seeker := r.NewSeeker()

			startPrefix := operation.MakePrefix(codePrefix, uint64(6))
			endPrefix := operation.MakePrefix(codePrefix, uint64(10))

			// Key 5 exists, but it is outside of specified range, so nil is returned.
			key := operation.MakePrefix(codePrefix, uint64(6))
			foundKey, found, err := seeker.SeekLE(startPrefix, endPrefix, key)
			require.NoError(t, err)
			require.False(t, found)
			require.Nil(t, foundKey)
		})
	})
}
