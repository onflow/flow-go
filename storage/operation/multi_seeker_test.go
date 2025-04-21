package operation_test

import (
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/badgerimpl"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestMultiSeeker(t *testing.T) {
	unittest.RunWithBadgerDBAndPebbleDB(t, func(bdb *badger.DB, pdb *pebble.DB) {
		// Insert the keys into the storage
		codePrefix := byte(1)
		badgerDBKeyParts := []uint64{1, 5}
		pebbleKeyParts := []uint64{9}

		// Store keys in BadgerDB
		err := badgerimpl.ToDB(bdb).WithReaderBatchWriter(func(rbw storage.ReaderBatchWriter) error {
			for _, keyPart := range badgerDBKeyParts {
				key := operation.MakePrefix(codePrefix, keyPart)
				err := rbw.Writer().Set(key, []byte{0x01})
				if err != nil {
					return err
				}
			}
			return nil
		})
		require.NoError(t, err)

		// Store keys in Pebble
		err = pebbleimpl.ToDB(pdb).WithReaderBatchWriter(func(rbw storage.ReaderBatchWriter) error {
			for _, keyPart := range pebbleKeyParts {
				key := operation.MakePrefix(codePrefix, keyPart)
				err := rbw.Writer().Set(key, []byte{0x01})
				if err != nil {
					return err
				}
			}
			return nil
		})
		require.NoError(t, err)

		r := operation.NewMultiReader(
			pebbleimpl.ToDB(pdb).Reader(),
			badgerimpl.ToDB(bdb).Reader(),
		)

		t.Run("key below start prefix", func(t *testing.T) {
			seeker := r.NewSeeker()

			key := operation.MakePrefix(codePrefix, uint64(4))
			startPrefix := operation.MakePrefix(codePrefix, uint64(5))

			_, err := seeker.SeekLE(startPrefix, key)
			require.Error(t, err)
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

		t.Run("seek key in first db (Pebble)", func(t *testing.T) {
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
		})

		t.Run("seek key in second db (BadgerDB)", func(t *testing.T) {
			seeker := r.NewSeeker()

			startPrefix := operation.MakePrefix(codePrefix)

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
	})
}
