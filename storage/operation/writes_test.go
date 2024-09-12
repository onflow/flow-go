package operation_test

import (
	"encoding/binary"
	"errors"
	"sync"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/bops"
	"github.com/onflow/flow-go/storage/operation/pops"
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

			reader := bops.ToReader(db)
			fn(t, reader, withWriterTx)
		})
	})

	t.Run("PebbleStorage", func(t *testing.T) {
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			withWriterTx := func(t *testing.T, writing func(storage.Writer) error) {
				writer := pops.NewReaderBatchWriter(db)
				require.NoError(t, writing(writer))
				require.NoError(t, writer.Commit())
			}

			reader := pops.ToReader(db)
			fn(t, reader, withWriterTx)
		})
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

// Verify multiple entities can be removed in one batch update
func TestBatchWrite(t *testing.T) {
	RunWithStorages(t, func(t *testing.T, r storage.Reader, withWriterTx WithWriter) {
		// Define multiple entities for batch insertion
		entities := []Entity{
			{ID: 1337},
			{ID: 42},
			{ID: 84},
		}

		// Batch write: insert multiple entities in a single transaction
		withWriterTx(t, func(writer storage.Writer) error {
			for _, e := range entities {
				if err := operation.Upsert(e.Key(), e)(writer); err != nil {
					return err
				}
			}
			return nil
		})

		// Verify that each entity can be read back
		for _, e := range entities {
			var readBack Entity
			require.NoError(t, operation.Retrieve(e.Key(), &readBack)(r))
			require.Equal(t, e, readBack, "expected retrieved value to match written value for entity ID %d", e.ID)
		}

		// Batch update: remove multiple entities in a single transaction
		withWriterTx(t, func(writer storage.Writer) error {
			for _, e := range entities {
				if err := operation.Remove(e.Key())(writer); err != nil {
					return err
				}
			}
			return nil
		})

		// Verify that each entity has been removed
		for _, e := range entities {
			var readBack Entity
			err := operation.Retrieve(e.Key(), &readBack)(r)
			require.True(t, errors.Is(err, storage.ErrNotFound), "expected not found error for entity ID %d after removal", e.ID)
		}
	})
}

func TestRemove(t *testing.T) {
	RunWithStorages(t, func(t *testing.T, r storage.Reader, withWriterTx WithWriter) {
		e := Entity{ID: 1337}

		var exists bool
		require.NoError(t, operation.Exists(e.Key(), &exists)(r))
		require.False(t, exists, "expected key to not exist")

		// Test delete nothing should return OK
		withWriterTx(t, operation.Remove(e.Key()))

		// Test write, delete, then read should return not found
		withWriterTx(t, operation.Upsert(e.Key(), e))

		require.NoError(t, operation.Exists(e.Key(), &exists)(r))
		require.True(t, exists, "expected key to exist")

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

func TestConcurrentRemove(t *testing.T) {
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

func TestRemoveRange(t *testing.T) {
	RunWithStorages(t, func(t *testing.T, r storage.Reader, withWriterTx WithWriter) {

		// Define the prefix
		prefix := []byte{0x10}

		// Create a range of keys around the boundaries of the prefix
		keys := [][]byte{
			// before prefix -> not included in range
			{0x09, 0xff},
			// within the prefix -> included in range
			{0x10, 0x00},
			{0x10, 0x50},
			{0x10, 0xff},
			// after end -> not included in range
			{0x11, 0x00},
			{0x1A, 0xff},
		}

		// Keys expected to be in the prefix range
		includeStart, includeEnd := 1, 3

		// Insert the keys into the storage
		withWriterTx(t, func(writer storage.Writer) error {
			for _, key := range keys {
				value := []byte{0x00} // value are skipped, doesn't matter
				err := operation.Upsert(key, value)(writer)
				if err != nil {
					return err
				}
			}
			return nil
		})

		// Remove the keys in the prefix range
		withWriterTx(t, operation.RemoveByPrefix(r, prefix))

		lg := unittest.Logger().With().Logger()
		// Verify that the keys in the prefix range have been removed
		for i, key := range keys {
			var exists bool
			require.NoError(t, operation.Exists(key, &exists)(r))
			lg.Info().Msgf("key %x exists: %t", key, exists)

			deleted := includeStart <= i && i <= includeEnd

			// deleted item should not exist
			require.Equal(t, !deleted, exists,
				"expected key %x to be %s", key, map[bool]string{true: "deleted", false: "not deleted"})
		}
	})

}
