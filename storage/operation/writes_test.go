package operation_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v4"
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

func TestRemoveDiskUsagePebble(t *testing.T) {
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

		// Use a timer to implement a timeout for wg.Wait()
		timeout := time.After(30 * time.Second)
		done := make(chan struct{})

		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// WaitGroup finished successfully
		case <-timeout:
			t.Fatal("Test timed out waiting for WAL files to be deleted")
		}

		// Verify the disk usage is reduced
		sizeAfter := getFolderSize(t, dir)
		require.Greater(t, sizeBefore, sizeAfter,
			fmt.Sprintf("expected disk usage to be reduced after compaction, before: %d, after: %d", sizeBefore, sizeAfter))
	})
}

func TestRemoveDiskUsageBadger(t *testing.T) {
	count := 10000
	// Configure Pebble DB with the event listener
	opts := &badger.Options{}

	dbtest.RunWithBadgerDB(t, opts, func(t *testing.T, r storage.Reader, withWriter dbtest.WithWriter, dir string, db *badger.DB) {
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
		err := db.RunValueLogGC(0.5)
		if err != nil {
			fmt.Printf("failed to run value log GC: %v\n", err)
		}

		// Use a timer to implement a timeout for wg.Wait()
		timeout := time.After(5 * time.Second)

		// WaitGroup finished successfully
		<-timeout

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

func TestRemoveByPrefix(t *testing.T) {
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

		// Verify that the keys in the prefix range have been removed
		for i, key := range keys {
			var exists bool
			require.NoError(t, operation.Exists(key, &exists)(r))
			t.Logf("key %x exists: %t", key, exists)

			deleted := includeStart <= i && i <= includeEnd

			// An item that was not deleted must exist
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
		err = operation.Iterate([]byte{keys[0][0]}, storage.PrefixUpperBound(keys[len(keys)-1]), func(key []byte) error {
			actual = append(actual, key)
			return nil
		})(r)
		require.NoError(t, err)
		require.Equal(t, expected, actual, "expected keys to match expected values")
	})
}

func TestRemoveByRange(t *testing.T) {
	dbtest.RunWithStorages(t, func(t *testing.T, r storage.Reader, withWriter dbtest.WithWriter) {

		startPrefix, endPrefix := []byte{0x10}, []byte{0x12}
		// Create a range of keys around the boundaries of the prefix
		keys := [][]byte{
			{0x09, 0xff},
			// within the range
			{0x10, 0x00},
			{0x10, 0x50},
			{0x10, 0xff},
			{0x11},
			{0x12},
			{0x12, 0x00},
			{0x12, 0xff},
			// after end -> not included in range
			{0x13},
			{0x1A, 0xff},
		}

		// Keys expected to be in the prefix range
		includeStart, includeEnd := 1, 7

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
		require.NoError(t, withWriter(operation.RemoveByRange(r, startPrefix, endPrefix)))

		// Verify that the keys in the prefix range have been removed
		for i, key := range keys {
			var exists bool
			require.NoError(t, operation.Exists(key, &exists)(r))
			t.Logf("key %x exists: %t", key, exists)

			deleted := includeStart <= i && i <= includeEnd

			// An item that was not deleted must exist
			require.Equal(t, !deleted, exists,
				"expected key %x to be %s", key, map[bool]string{true: "deleted", false: "not deleted"})
		}
	})
}

func TestRemoveFrom(t *testing.T) {
	dbtest.RunWithStorages(t, func(t *testing.T, r storage.Reader, withWriter dbtest.WithWriter) {

		// Define the prefix
		prefix := []byte{0xff}

		// Create a range of keys around the boundaries of the prefix
		keys := [][]byte{
			{0x10, 0x00},
			{0xff},
			{0xff, 0x00},
			{0xff, 0xff},
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

		// Verify that the keys in the prefix range have been removed
		for i, key := range keys {
			var exists bool
			require.NoError(t, operation.Exists(key, &exists)(r))
			t.Logf("key %x exists: %t", key, exists)

			deleted := includeStart <= i && i <= includeEnd

			// An item that was not deleted must exist
			require.Equal(t, !deleted, exists,
				fmt.Errorf("a key %x should be deleted (%v), but actually exists (%v)", key, deleted, exists))
		}
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

func TestBadgerDeletionAndGC(t *testing.T) {
	dir := "./tmp_badger_test"
	_ = os.RemoveAll(dir)

	opts := badger.DefaultOptions(dir)
	opts.ValueThreshold = 1
	opts.Logger = nil // Silence logs
	db, err := badger.Open(opts)
	if err != nil {
		t.Fatalf("Failed to open DB: %v", err)
	}
	defer func() {
		db.Close()
		_ = os.RemoveAll(dir)
	}()

	// Step 1: Insert large values (e.g. 1MB each)
	value := make([]byte, 1<<20) // 1MB
	for i := 0; i < 10; i++ {
		err := db.Update(func(txn *badger.Txn) error {
			return txn.Set([]byte(fmt.Sprintf("key-%d", i)), value)
		})
		if err != nil {
			t.Fatalf("Failed to insert key: %v", err)
		}
	}

	sizeBeforeDelete := getFolderSize(t, dir)
	t.Logf("Size before delete: %d bytes", sizeBeforeDelete)

	// Step 2: Delete all keys
	for i := 0; i < 10; i++ {
		err := db.Update(func(txn *badger.Txn) error {
			return txn.Delete([]byte(fmt.Sprintf("key-%d", i)))
		})
		if err != nil {
			t.Fatalf("Failed to delete key: %v", err)
		}
	}

	// Step 3: Wait a bit, check size again (expect no drop)
	time.Sleep(2 * time.Second)
	sizeAfterDelete := getFolderSize(t, dir)
	t.Logf("Size after delete: %d bytes", sizeAfterDelete)

	if sizeAfterDelete < sizeBeforeDelete {
		t.Errorf("Expected no disk space reclaimed yet, but it was!")
	}

	// Step 4: Run Value Log GC
	ranGC := false
	for {
		err := db.RunValueLogGC(0.1)
		if err == nil {
			ranGC = true
			continue
		}
		break
	}
	if !ranGC {
		t.Errorf("Value log GC did not run at all")
	}

	// Step 5: Final disk size check (expect drop)
	time.Sleep(2 * time.Second)
	sizeAfterGC := getFolderSize(t, dir)
	t.Logf("Size after GC: %d bytes", sizeAfterGC)

	if sizeAfterGC >= sizeAfterDelete {
		t.Errorf("Expected disk space to drop after GC, but it didn't")
	}
}
func TestValueLogReclaim(t *testing.T) {
	// Use a temporary directory for the Badger database
	dbDir := t.TempDir()

	// Open Badger with default options, but ValueThreshold=1 (all values in value log) [oai_citation_attribution:3‡chromium.googlesource.com](https://chromium.googlesource.com/external/github.com/dgraph-io/badger/+/c2e611ac7cc63c80e16d10a53cb30fa56482d246/db2_test.go#:~:text=opt,Txn%29%20error)
	opts := badger.DefaultOptions(dbDir)
	opts.ValueThreshold = 1
	db, err := badger.Open(opts)
	if err != nil {
		t.Fatal(err)
	}

	// Step 1: Insert large values (e.g., 1 MB each) to ensure the DB on disk grows
	value := bytes.Repeat([]byte("X"), 1<<20) // 1 MB of data
	numKeys := 10
	for i := 0; i < numKeys; i++ {
		key := []byte(fmt.Sprintf("key-%06d", i))
		err := db.Update(func(txn *badger.Txn) error {
			return txn.Set(key, value)
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// Flush everything to disk by closing and reopening (ensure vlog file is finalized)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	// Step 2: Record directory size before deletion
	beforeSize := getFolderSize(t, dbDir)
	t.Logf("Directory size before deletion: %d bytes", beforeSize)

	// Reopen the DB to perform deletions
	db, err = badger.Open(opts)
	if err != nil {
		t.Fatal(err)
	}

	// Step 3: Delete all the inserted data (each key)
	for i := 0; i < numKeys; i++ {
		key := []byte(fmt.Sprintf("key-%06d", i))
		err := db.Update(func(txn *badger.Txn) error {
			return txn.Delete(key)
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// Step 4: Wait briefly for any background flush/compaction to finish
	time.Sleep(1 * time.Second)

	// Close and reopen to ensure deletion tombstones are flushed to disk.
	// (Badger won't GC the latest value log file while it's active [oai_citation_attribution:4‡pkg.go.dev](https://pkg.go.dev/github.com/dgraph-io/badger#:~:text=Badger%27s%20LSM%20tree%20automatically%20discards,older%2Finvalid%20versions%20of%20keys).)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	// Record directory size after deletion (before GC)
	afterDelSize := getFolderSize(t, dbDir)
	t.Logf("Directory size after deletion (before GC): %d bytes", afterDelSize)

	// Step 5: Open DB again and run value log GC repeatedly until no more can be reclaimed
	db, err = badger.Open(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close() // ensure DB is closed at the end of the test

	for {
		err := db.RunValueLogGC(0.1) // 10% discard ratio for aggressive GC
		if err == nil {
			// GC succeeded in reclaiming a file, continue to try again
			continue
		}
		if err == badger.ErrNoRewrite {
			// No more log files could be GCed (nothing left to reclaim)
			break
		}
		// An unexpected error occurred
		t.Fatalf("Unexpected error during value log GC: %v", err)
	}

	// Step 6: Record the directory size after running GC
	afterGCSize := getFolderSize(t, dbDir)
	t.Logf("Directory size after GC: %d bytes", afterGCSize)

	// Step 7: Assert that the size after GC is significantly smaller than before deletion
	if afterGCSize*2 > beforeSize { // after GC should be < 50% of before
		t.Errorf("Disk usage not reduced enough: before=%d bytes, after GC=%d bytes", beforeSize, afterGCSize)
	}
}
