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

func TestMultiIterator(t *testing.T) {
	unittest.RunWithBadgerDBAndPebbleDB(t, func(bdb *badger.DB, pdb *pebble.DB) {
		prefix := byte(0x01)

		const lowBound = 0x00
		const highBound = 0xff
		const keyCount = highBound - lowBound + 1

		keys := make([][]byte, 0, keyCount)
		values := make([][]byte, 0, keyCount)
		for i := lowBound; i < highBound+1; i++ {
			keys = append(keys, operation.MakePrefix(prefix, byte(i)))
			values = append(values, []byte{byte(i)})
		}

		// Store first half of key-value pairs in BadgerDB
		err := badgerimpl.ToDB(bdb).WithReaderBatchWriter(func(rbw storage.ReaderBatchWriter) error {
			for i := 0; i < len(keys)/2; i++ {
				err := rbw.Writer().Set(keys[i], values[i])
				if err != nil {
					return err
				}
			}
			return nil
		})
		require.NoError(t, err)

		// Store second half of key-value pairs in Pebble
		err = pebbleimpl.ToDB(pdb).WithReaderBatchWriter(func(rbw storage.ReaderBatchWriter) error {
			for i := len(keys) / 2; i < len(keys); i++ {
				err := rbw.Writer().Set(keys[i], values[i])
				if err != nil {
					return err
				}
			}
			return nil
		})
		require.NoError(t, err)

		preader := pebbleimpl.ToDB(pdb).Reader()

		breader := badgerimpl.ToDB(bdb).Reader()

		reader := operation.NewMultiReader(preader, breader)

		t.Run("not found, less than lower bound", func(t *testing.T) {
			startPrefix := []byte{0x00}
			endPrefix := []byte{0x00, 0xff}

			it, err := reader.NewIter(startPrefix, endPrefix, storage.DefaultIteratorOptions())
			require.NoError(t, err)

			defer it.Close()

			require.False(t, it.First())
			require.False(t, it.Valid())

			it.Next()
			require.False(t, it.Valid())
		})

		t.Run("not found, higher than upper bound", func(t *testing.T) {
			startPrefix := []byte{0x02}
			endPrefix := []byte{0x02, 0xff}

			it, err := reader.NewIter(startPrefix, endPrefix, storage.DefaultIteratorOptions())
			require.NoError(t, err)

			defer it.Close()

			require.False(t, it.First())
			require.False(t, it.Valid())

			it.Next()
			require.False(t, it.Valid())
		})

		t.Run("found in second db", func(t *testing.T) {
			startPrefix := []byte{0x01}
			endPrefix := []byte{0x01, 0x0f}
			expectedCount := 16

			it, err := reader.NewIter(startPrefix, endPrefix, storage.DefaultIteratorOptions())
			require.NoError(t, err)

			defer it.Close()

			i := 0
			for it.First(); it.Valid(); it.Next() {
				item := it.IterItem()

				require.Equal(t, keys[i], item.Key())

				err = item.Value(func(val []byte) error {
					require.Equal(t, values[i], val)
					return nil
				})
				require.NoError(t, err)

				i++
			}
			require.Equal(t, expectedCount, i)
		})

		t.Run("found in first db", func(t *testing.T) {
			startPrefix := []byte{0x01, 0xf0}
			endPrefix := []byte{0x01, 0xff}
			expectedCount := 16

			it, err := reader.NewIter(startPrefix, endPrefix, storage.DefaultIteratorOptions())
			require.NoError(t, err)

			defer it.Close()

			count := 0
			i := len(keys) - expectedCount
			for it.First(); it.Valid(); it.Next() {
				item := it.IterItem()

				require.Equal(t, keys[i], item.Key())

				err = item.Value(func(val []byte) error {
					require.Equal(t, values[i], val)
					return nil
				})
				require.NoError(t, err)

				i++
				count++
			}
			require.Equal(t, expectedCount, count)
		})

		t.Run("found in both db", func(t *testing.T) {
			startPrefix := []byte{0x01, 0x0f}
			endPrefix := []byte{0x01, 0xf0}
			expectedCount := len(keys) - 15 - 15

			it, err := reader.NewIter(startPrefix, endPrefix, storage.DefaultIteratorOptions())
			require.NoError(t, err)

			defer it.Close()

			count := 0
			i := 15
			for it.First(); it.Valid(); it.Next() {
				item := it.IterItem()

				require.Equal(t, keys[i], item.Key())

				err = item.Value(func(val []byte) error {
					require.Equal(t, values[i], val)
					return nil
				})
				require.NoError(t, err)

				i++
				count++
			}
			require.Equal(t, expectedCount, count)
		})
	})
}
