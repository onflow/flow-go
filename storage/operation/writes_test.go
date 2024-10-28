package operation_test

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestReadWrite(t *testing.T) {
	dbtest.RunWithStorages(t, func(t *testing.T, r storage.Reader, withWriter dbtest.WithWriter) {
		e := Entity{ID: 1337}

		// Test read nothing should return not found
		var item Entity
		err := operation.Retrieve(e.Key(), &item)(r)
		require.True(t, errors.Is(err, storage.ErrNotFound), "expected not found error")

		require.NoError(t, withWriter(operation.Upsert(e.Key(), e)))

		var readBack Entity
		require.NoError(t, operation.Retrieve(e.Key(), &readBack)(r))
		require.Equal(t, e, readBack, "expected retrieved value to match written value")

		// Test write again should overwrite
		newEntity := Entity{ID: 42}
		require.NoError(t, withWriter(operation.Upsert(e.Key(), newEntity)))

		require.NoError(t, operation.Retrieve(e.Key(), &readBack)(r))
		require.Equal(t, newEntity, readBack, "expected overwritten value to be retrieved")

		// Test write should not overwrite a different key
		anotherEntity := Entity{ID: 84}
		require.NoError(t, withWriter(operation.Upsert(anotherEntity.Key(), anotherEntity)))

		var anotherReadBack Entity
		require.NoError(t, operation.Retrieve(anotherEntity.Key(), &anotherReadBack)(r))
		require.Equal(t, anotherEntity, anotherReadBack, "expected different key to return different value")
	})
}

func TestReadWriteMalformed(t *testing.T) {
	dbtest.RunWithStorages(t, func(t *testing.T, r storage.Reader, withWriter dbtest.WithWriter) {
		e := Entity{ID: 1337}
		ue := UnencodeableEntity(e)

		// Test write should return encoding error
		require.NoError(t, withWriter(func(writer storage.Writer) error {
			err := operation.Upsert(e.Key(), ue)(writer)
			require.Contains(t, err.Error(), errCantEncode.Error(), "expected encoding error")
			return nil
		}))

		// Test read should return decoding error
		var exists bool
		require.NoError(t, operation.Exists(e.Key(), &exists)(r))
		require.False(t, exists, "expected key to not exist")
	})
}

// Verify multiple entities can be removed in one batch update
func TestBatchWrite(t *testing.T) {
	dbtest.RunWithStorages(t, func(t *testing.T, r storage.Reader, withWriter dbtest.WithWriter) {
		// Define multiple entities for batch insertion
		entities := []Entity{
			{ID: 1337},
			{ID: 42},
			{ID: 84},
		}

		// Batch write: insert multiple entities in a single transaction
		require.NoError(t, withWriter(func(writer storage.Writer) error {
			for _, e := range entities {
				if err := operation.Upsert(e.Key(), e)(writer); err != nil {
					return err
				}
			}
			return nil
		}))

		// Verify that each entity can be read back
		for _, e := range entities {
			var readBack Entity
			require.NoError(t, operation.Retrieve(e.Key(), &readBack)(r))
			require.Equal(t, e, readBack, "expected retrieved value to match written value for entity ID %d", e.ID)
		}

		// Batch update: remove multiple entities in a single transaction
		require.NoError(t, withWriter(func(writer storage.Writer) error {
			for _, e := range entities {
				if err := operation.Remove(e.Key())(writer); err != nil {
					return err
				}
			}
			return nil
		}))

		// Verify that each entity has been removed
		for _, e := range entities {
			var readBack Entity
			err := operation.Retrieve(e.Key(), &readBack)(r)
			require.True(t, errors.Is(err, storage.ErrNotFound), "expected not found error for entity ID %d after removal", e.ID)
		}
	})
}

func TestRemove(t *testing.T) {
	dbtest.RunWithStorages(t, func(t *testing.T, r storage.Reader, withWriter dbtest.WithWriter) {
		e := Entity{ID: 1337}

		var exists bool
		require.NoError(t, operation.Exists(e.Key(), &exists)(r))
		require.False(t, exists, "expected key to not exist")

		// Test delete nothing should return OK
		require.NoError(t, withWriter(operation.Remove(e.Key())))

		// Test write, delete, then read should return not found
		require.NoError(t, withWriter(operation.Upsert(e.Key(), e)))

		require.NoError(t, operation.Exists(e.Key(), &exists)(r))
		require.True(t, exists, "expected key to exist")

		require.NoError(t, withWriter(operation.Remove(e.Key())))

		var item Entity
		err := operation.Retrieve(e.Key(), &item)(r)
		require.True(t, errors.Is(err, storage.ErrNotFound), "expected not found error after delete")
	})
}

func TestRemoveDiskUsage(t *testing.T) {
	count := 10000
	wg := sync.WaitGroup{}
	// 10000 chunk data packs will produce 4 log files
	// Wait for the 4 log file to be deleted
	wg.Add(4)

	// Create an event listener to monitor compaction events
	listener := pebble.EventListener{
		// Capture when compaction ends
		WALDeleted: func(info pebble.WALDeleteInfo) {
			wg.Done()
		},
	}

	// Configure Pebble DB with the event listener
	opts := &pebble.Options{
		MemTableSize:  64 << 20, // required for rotating WAL
		EventListener: &listener,
	}

	dbtest.RunWithPebbleDB(t, opts, func(t *testing.T, r storage.Reader, withWriter dbtest.WithWriter, dir string, db *pebble.DB) {
		items := make([]*flow.ChunkDataPack, count)

		// prefix is needed for defining the key range for compaction
		prefix := []byte{1}
		getKey := func(c *flow.ChunkDataPack) []byte {
			return append(prefix, c.ChunkID[:]...)
		}

		for i := 0; i < count; i++ {
			chunkID := unittest.IdentifierFixture()
			chunkDataPack := unittest.ChunkDataPackFixture(chunkID)
			items[i] = chunkDataPack
		}

		// Insert 100 entities
		require.NoError(t, withWriter(func(writer storage.Writer) error {
			for i := 0; i < count; i++ {
				if err := operation.Upsert(getKey(items[i]), items[i])(writer); err != nil {
					return err
				}
			}
			return nil
		}))
		sizeBefore := getFolderSize(t, dir)

		// Remove all entities
		require.NoError(t, withWriter(func(writer storage.Writer) error {
			for i := 0; i < count; i++ {
				if err := operation.Remove(getKey(items[i]))(writer); err != nil {
					return err
				}
			}
			return nil
		}))

		// Trigger compaction
		require.NoError(t, db.Compact(prefix, []byte{2}, true))
		wg.Wait()

		// Verify the disk usage is reduced
		sizeAfter := getFolderSize(t, dir)
		require.Greater(t, sizeBefore, sizeAfter,
			fmt.Sprintf("expected disk usage to be reduced after compaction, before: %d, after: %d", sizeBefore, sizeAfter))
	})
}

func TestConcurrentWrite(t *testing.T) {
	dbtest.RunWithStorages(t, func(t *testing.T, r storage.Reader, withWriter dbtest.WithWriter) {
		var wg sync.WaitGroup
		numWrites := 10 // number of concurrent writes

		for i := 0; i < numWrites; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				e := Entity{ID: uint64(i)}

				// Simulate a concurrent write to a different key
				require.NoError(t, withWriter(operation.Upsert(e.Key(), e)))

				var readBack Entity
				require.NoError(t, operation.Retrieve(e.Key(), &readBack)(r))
				require.Equal(t, e, readBack, "expected retrieved value to match written value for key %d", i)
			}(i)
		}

		wg.Wait() // Wait for all goroutines to finish
	})
}

func TestConcurrentRemove(t *testing.T) {
	dbtest.RunWithStorages(t, func(t *testing.T, r storage.Reader, withWriter dbtest.WithWriter) {
		var wg sync.WaitGroup
		numDeletes := 10 // number of concurrent deletions

		// First, insert entities to be deleted concurrently
		for i := 0; i < numDeletes; i++ {
			e := Entity{ID: uint64(i)}
			require.NoError(t, withWriter(operation.Upsert(e.Key(), e)))
		}

		// Now, perform concurrent deletes
		for i := 0; i < numDeletes; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				e := Entity{ID: uint64(i)}

				// Simulate a concurrent delete
				require.NoError(t, withWriter(operation.Remove(e.Key())))

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
	dbtest.RunWithStorages(t, func(t *testing.T, r storage.Reader, withWriter dbtest.WithWriter) {

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
		require.NoError(t, withWriter(func(writer storage.Writer) error {
			for _, key := range keys {
				value := []byte{0x00} // value are skipped, doesn't matter
				err := operation.Upsert(key, value)(writer)
				if err != nil {
					return err
				}
			}
			return nil
		}))

		// Remove the keys in the prefix range
		require.NoError(t, withWriter(operation.RemoveByPrefix(r, prefix)))

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

		// Verify that after the removal, Traverse the removed prefix would return nothing
		removedKeys := make([]string, 0)
		err := operation.Traverse(prefix, operation.KeyOnlyIterateFunc(func(key []byte) error {
			removedKeys = append(removedKeys, fmt.Sprintf("%x", key))
			return nil
		}), storage.DefaultIteratorOptions())(r)
		require.NoError(t, err)
		require.Len(t, removedKeys, 0, "expected no entries to be found when traversing the removed prefix")

		// Verify that after the removal, Iterate over all keys should only return keys outside the prefix range
		expected := [][]byte{
			{0x09, 0xff},
			{0x11, 0x00},
			{0x1A, 0xff},
		}

		actual := make([][]byte, 0)
		err = operation.Iterate([]byte{keys[0][0]}, operation.PrefixUpperBound(keys[len(keys)-1]), operation.KeyOnlyIterateFunc(func(key []byte) error {
			actual = append(actual, key)
			return nil
		}), storage.DefaultIteratorOptions())(r)
		require.NoError(t, err)
		require.Equal(t, expected, actual, "expected keys to match expected values")
	})
}

// verify if iterating each key and writing new keys concurrently, the iterator will visit all keys once
func TestIterateAndWriteConcurrently(t *testing.T) {
	dbtest.RunWithStorages(t, func(t *testing.T, r storage.Reader, withWriter dbtest.WithWriter) {
		prefix := []byte{0x10}

		storeKey := func(e Entity) []byte {
			return append(prefix, e.Key()...)
		}

		// adding some data to database
		count := 100
		for i := 0; i < count; i++ {
			e := Entity{ID: uint64(i)}
			require.NoError(t, withWriter(operation.Upsert(storeKey(e), e)))
		}

		// iterate over all keys and write new keys concurrently
		var wg sync.WaitGroup
		visitedKeys := make(map[string]struct{})
		newKeysCount := 50

		// Goroutine for iteration
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := operation.Iterate(prefix, operation.PrefixUpperBound(prefix), operation.KeyOnlyIterateFunc(func(key []byte) error {
				_, visited := visitedKeys[string(key)]
				require.False(t, visited)
				visitedKeys[string(key)] = struct{}{}
				return nil
			}), storage.DefaultIteratorOptions())(r)
			require.NoError(t, err)
		}()

		// Goroutine for writing new keys
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < newKeysCount; i++ {
				e := Entity{ID: uint64(count + i)}
				require.NoError(t, withWriter(operation.Upsert(storeKey(e), e)))
			}
		}()

		wg.Wait()

		// Verify that all keys (including new ones) were visited
		require.True(t, len(visitedKeys) >= count, "expected all keys to be visited")
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

type UnencodeableEntity Entity

var errCantEncode = fmt.Errorf("encoding not supported")
var errCantDecode = fmt.Errorf("decoding not supported")

func (a UnencodeableEntity) MarshalJSON() ([]byte, error) {
	return nil, errCantEncode
}

func (a *UnencodeableEntity) UnmarshalJSON(b []byte) error {
	return errCantDecode
}

func (a UnencodeableEntity) MarshalMsgpack() ([]byte, error) {
	return nil, errCantEncode
}

func (a UnencodeableEntity) UnmarshalMsgpack(b []byte) error {
	return errCantDecode
}

func getFolderSize(t testing.TB, dir string) int64 {
	var size int64
	require.NoError(t, filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				fmt.Printf("warning: could not get file info for %s: %v\n", path, err)
				return nil
			}

			// Add the file size to total
			size += info.Size()
		}
		return nil
	}))

	return size
}
