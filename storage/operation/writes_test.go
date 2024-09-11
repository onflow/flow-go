package operation_test

import (
	"encoding/binary"
	"errors"
	"sync"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage"
	bops "github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/utils/unittest"
)

type WithWriter func(*testing.T, func(storage.Writer) error)

func RunWithStorages(t *testing.T, fn func(*testing.T, storage.Reader, WithWriter)) {
	t.Run("BadgerStorage", func(t *testing.T) {
		unittest.RunWithBadgerDB(t, func(db *badger.DB) {
			withWriterTx := func(t *testing.T, writing func(storage.Writer) error) {
				writer := bops.NewReaderBatchWriter(db)
				require.NoError(t, writing(writer))
				require.NoError(t, writer.Commit())
			}

			// TODO: make NewReader
			reader := bops.ToReader(db)
			fn(t, reader, withWriterTx)
		})
	})

	t.Run("PebbleStorage", func(t *testing.T) {
		// unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		// })
	})
}

type Entity struct {
	ID uint64
}

func (e Entity) Key() []byte {
	byteSlice := make([]byte, 8) // uint64 is 8 bytes
	binary.BigEndian.PutUint64(byteSlice, e.ID)
	return byteSlice
}

func TestReadWrite(t *testing.T) {
	RunWithStorages(t, func(t *testing.T, r storage.Reader, withWriterTx WithWriter) {
		e := Entity{ID: 1337}

		// Test read nothing should return not found
		var item Entity
		err := operation.Retrieve(e.Key(), &item)(r)
		require.True(t, errors.Is(err, storage.ErrNotFound), "expected not found error")

		withWriterTx(t, operation.Upsert(e.Key(), e))

		var readBack Entity
		require.NoError(t, operation.Retrieve(e.Key(), &readBack)(r))
		require.Equal(t, e, readBack, "expected retrieved value to match written value")

		// Test write again should overwrite
		newEntity := Entity{ID: 42}
		withWriterTx(t, operation.Upsert(e.Key(), newEntity))

		require.NoError(t, operation.Retrieve(e.Key(), &readBack)(r))
		require.Equal(t, newEntity, readBack, "expected overwritten value to be retrieved")

		// Test write should not overwrite a different key
		anotherEntity := Entity{ID: 84}
		withWriterTx(t, operation.Upsert(anotherEntity.Key(), anotherEntity))

		var anotherReadBack Entity
		require.NoError(t, operation.Retrieve(anotherEntity.Key(), &anotherReadBack)(r))
		require.Equal(t, anotherEntity, anotherReadBack, "expected different key to return different value")
	})
}

func TestDelete(t *testing.T) {
	RunWithStorages(t, func(t *testing.T, r storage.Reader, withWriterTx WithWriter) {
		e := Entity{ID: 1337}

		// Test delete nothing should return OK
		withWriterTx(t, operation.Remove(e.Key()))

		// Test write, delete, then read should return not found
		withWriterTx(t, operation.Upsert(e.Key(), e))
		withWriterTx(t, operation.Remove(e.Key()))

		var item Entity
		err := operation.Retrieve(e.Key(), &item)(r)
		require.True(t, errors.Is(err, storage.ErrNotFound), "expected not found error after delete")
	})
}

func TestConcurrentWrite(t *testing.T) {
	RunWithStorages(t, func(t *testing.T, r storage.Reader, withWriterTx WithWriter) {
		var wg sync.WaitGroup
		numWrites := 10 // number of concurrent writes

		for i := 0; i < numWrites; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				e := Entity{ID: uint64(i)}

				// Simulate a concurrent write to a different key
				withWriterTx(t, operation.Upsert(e.Key(), e))

				var readBack Entity
				require.NoError(t, operation.Retrieve(e.Key(), &readBack)(r))
				require.Equal(t, e, readBack, "expected retrieved value to match written value for key %d", i)
			}(i)
		}

		wg.Wait() // Wait for all goroutines to finish
	})
}

func TestConcurrentDelete(t *testing.T) {
	RunWithStorages(t, func(t *testing.T, r storage.Reader, withWriterTx WithWriter) {
		var wg sync.WaitGroup
		numDeletes := 10 // number of concurrent deletions

		// First, insert entities to be deleted concurrently
		for i := 0; i < numDeletes; i++ {
			e := Entity{ID: uint64(i)}
			withWriterTx(t, operation.Upsert(e.Key(), e))
		}

		// Now, perform concurrent deletes
		for i := 0; i < numDeletes; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				e := Entity{ID: uint64(i)}

				// Simulate a concurrent delete
				withWriterTx(t, operation.Remove(e.Key()))

				// Check that the item is no longer retrievable
				var item Entity
				err := operation.Retrieve(e.Key(), &item)(r)
				require.True(t, errors.Is(err, storage.ErrNotFound), "expected not found error after delete for key %d", i)
			}(i)
		}

		wg.Wait() // Wait for all goroutines to finish
	})
}

func TestIterateKeysWithPrefixRange(t *testing.T) {
}

func TestTraverseKeysWithPrefix(t *testing.T) {
}

func TestFindHighestAtOrBelow(t *testing.T) {
}
