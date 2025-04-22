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

func TestMultiReader(t *testing.T) {
	unittest.RunWithBadgerDBAndPebbleDB(t, func(bdb *badger.DB, pdb *pebble.DB) {
		key1 := []byte{0x01, 0x01}
		value1 := []byte{0x01}

		key2 := []byte{0x01, 0x02}
		value2 := []byte{0x02}

		key3 := []byte{0x01, 0x03}
		value3a := []byte{0x03, 0x00}
		value3b := []byte{0x03, 0x01}

		notFoundKey := []byte{0x01, 0x04}

		// Store {key1, value1} and {key3, value3a} in Pebble
		err := pebbleimpl.ToDB(pdb).WithReaderBatchWriter(func(rbw storage.ReaderBatchWriter) error {
			err := rbw.Writer().Set(key1, value1)
			if err != nil {
				return err
			}
			return rbw.Writer().Set(key3, value3a)
		})
		require.NoError(t, err)

		// Store {key2, value2} and {key3, value3b} in BadgerDB
		err = badgerimpl.ToDB(bdb).WithReaderBatchWriter(func(rbw storage.ReaderBatchWriter) error {
			err := rbw.Writer().Set(key2, value2)
			if err != nil {
				return err
			}
			return rbw.Writer().Set(key3, value3b)
		})
		require.NoError(t, err)

		reader, err := operation.NewMultiReader(pebbleimpl.ToDB(pdb).Reader(), badgerimpl.ToDB(bdb).Reader())
		require.NoError(t, err)

		t.Run("not found", func(t *testing.T) {
			value, closer, err := reader.Get(notFoundKey)
			require.Equal(t, 0, len(value))
			require.ErrorIs(t, err, storage.ErrNotFound)

			closer.Close()
		})

		t.Run("in first db", func(t *testing.T) {
			value, closer, err := reader.Get(key1)
			require.Equal(t, value1, value)
			require.NoError(t, err)

			closer.Close()
		})

		t.Run("in second db", func(t *testing.T) {
			value, closer, err := reader.Get(key2)
			require.Equal(t, value2, value)
			require.NoError(t, err)

			closer.Close()
		})

		t.Run("in both db", func(t *testing.T) {
			value, closer, err := reader.Get(key3)
			require.Equal(t, value3a, value)
			require.NoError(t, err)

			closer.Close()
		})
	})
}
