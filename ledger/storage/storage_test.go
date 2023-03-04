package storage_test

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/storage"
)

func TestStorageImplementations(t *testing.T) {
	t.Run("test in_mem_storage", func(t *testing.T) {
		testStorageImplementation(
			t,
			func() ledger.Storage {
				return storage.NewInMemStorage()
			},
		)
	})
	t.Run("test pebble_storage", func(t *testing.T) {
		testStorageImplementation(
			t,
			func() ledger.Storage {
				s, err := storage.NewPebbleStorage(
					storage.TestPebleStorageOptions())
				require.NoError(t, err)
				return s
			})
	})
}

func testStorageImplementation(
	t *testing.T,
	newStorage func() ledger.Storage,
) {
	test := func(message string, test func(t *testing.T, storage ledger.Storage)) {
		s := newStorage()
		defer func(s ledger.Storage) {
			if s, ok := s.(io.Closer); ok {
				require.NoError(t, s.Close())
			}
		}(s)

		t.Run(message, func(t *testing.T) {
			test(t, s)
		})
	}

	test("get non-existent errors", func(t *testing.T, storage ledger.Storage) {
		key := keyBytes()
		value, err := storage.Get(key)
		require.Error(t, err)
		require.ErrorIs(t, err, ledger.ErrStorageMissingKeys{})
		require.Len(t, value, 0)

	})

	test("set does not fail", func(t *testing.T, storage ledger.Storage) {
		key := keyBytes()
		value := valueBytes()
		err := storage.SetMul(map[hash.Hash][]byte{key: value})
		require.NoError(t, err)
	})

	test("set then get does not fail", func(t *testing.T, storage ledger.Storage) {
		key := keyBytes()
		value := valueBytes()
		err := storage.SetMul(map[hash.Hash][]byte{key: value})
		require.NoError(t, err)
		gotValue, err := storage.Get(key)
		require.NoError(t, err)

		require.Equal(t, value, gotValue)
	})

	test("set empty then get nil", func(t *testing.T, storage ledger.Storage) {
		key := keyBytes()
		value := make([]byte, 0)
		err := storage.SetMul(map[hash.Hash][]byte{key: value})
		require.NoError(t, err)
		gotValue, err := storage.Get(key)

		require.NoError(t, err)
		require.Equal(t, value, gotValue)
	})

	test("set then set overrides", func(t *testing.T, storage ledger.Storage) {
		key := keyBytes()
		value1 := valueBytes()
		value2 := valueBytes()
		err := storage.SetMul(map[hash.Hash][]byte{key: value1})
		require.NoError(t, err)
		err = storage.SetMul(map[hash.Hash][]byte{key: value2})
		require.NoError(t, err)
		gotValue, err := storage.Get(key)
		require.NoError(t, err)

		require.Equal(t, value2, gotValue)
	})

	test("set then get twice", func(t *testing.T, storage ledger.Storage) {
		key := keyBytes()
		value := valueBytes()
		err := storage.SetMul(map[hash.Hash][]byte{key: value})
		require.NoError(t, err)
		gotValue1, err := storage.Get(key)
		require.NoError(t, err)
		gotValue2, err := storage.Get(key)
		require.NoError(t, err)

		require.Equal(t, value, gotValue1)
		require.Equal(t, value, gotValue2)
	})

	test("SetMul sets many", func(t *testing.T, storage ledger.Storage) {
		key1 := keyBytes()
		key2 := keyBytes()
		value1 := valueBytes()
		value2 := valueBytes()
		err := storage.SetMul(map[hash.Hash][]byte{key1: value1, key2: value2})

		require.NoError(t, err)
		gotValue1, err := storage.Get(key1)
		require.NoError(t, err)
		gotValue2, err := storage.Get(key2)
		require.NoError(t, err)

		require.Equal(t, value1, gotValue1)
		require.Equal(t, value2, gotValue2)
	})

	test("GetMul gets many", func(t *testing.T, storage ledger.Storage) {
		key1 := keyBytes()
		key2 := keyBytes()
		value1 := valueBytes()
		value2 := valueBytes()
		err := storage.SetMul(map[hash.Hash][]byte{key1: value1, key2: value2})
		require.NoError(t, err)
		values, err := storage.GetMul([]hash.Hash{key1, key2})
		require.NoError(t, err)

		gotValue1, gotValue2 := values[0], values[1]

		require.Equal(t, value1, gotValue1)
		require.Equal(t, value2, gotValue2)
	})

	// if this test is flaky, something is wrong with the storage implementation
	test("SetMul uses transactions", func(t *testing.T, storage ledger.Storage) {
		key1 := keyBytes()
		key2 := keyBytes()
		value1 := valueBytes()
		value2 := valueBytes()

		repetitions := 256

		writesDone := make(chan error)
		readsDone := make(chan error)

		// set once so the values are not empty on first read
		err := storage.SetMul(map[hash.Hash][]byte{key1: value1, key2: value1})
		require.NoError(t, err)

		go func() {
			var err error
			defer func() {
				writesDone <- err
			}()

			for i := 0; i < repetitions; i++ {
				err := storage.SetMul(map[hash.Hash][]byte{key1: value1, key2: value1})
				if err != nil {
					return
				}
				err = storage.SetMul(map[hash.Hash][]byte{key1: value2, key2: value2})
				if err != nil {
					return
				}
			}
		}()

		go func() {
			var err error
			defer func() {
				readsDone <- err
			}()

			for i := 0; i < repetitions; i++ {
				var values [][]byte
				values, err = storage.GetMul([]hash.Hash{key1, key2})
				if err != nil {
					return
				}

				gotValue1, gotValue2 := values[0], values[1]

				// values are set in a transaction, so they should be always the same
				if !bytes.Equal(gotValue1, gotValue2) {
					err = fmt.Errorf("values are different, writes are not transactional")
					return
				}
			}
		}()

		err = <-readsDone
		require.NoError(t, err)
		err = <-writesDone
		require.NoError(t, err)
	})
}

func keyBytes() hash.Hash {
	h, _ := hash.ToHash(someBytes(len(hash.Hash{})))
	return h
}

func valueBytes() []byte {
	return someBytes(1048)
}

func someBytes(l int) []byte {
	b := make([]byte, l)
	_, _ = rand.Read(b)
	return b
}
