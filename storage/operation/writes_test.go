package operation_test

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cockroachdb/pebble/v2"
	"github.com/dgraph-io/badger/v2"
	"github.com/dgraph-io/badger/v2/options"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/badgerimpl"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestReadWrite(t *testing.T) {
	dbtest.RunWithStorages(t, func(t *testing.T, r storage.Reader, withWriter dbtest.WithWriter) {
		e := Entity{ID: 1337}

		// Test read nothing should return not found
		var item Entity
		err := operation.RetrieveByKey(r, e.Key(), &item)
		require.True(t, errors.Is(err, storage.ErrNotFound), "expected not found error")

		require.NoError(t, withWriter(operation.Upsert(e.Key(), e)))

		var readBack Entity
		require.NoError(t, operation.RetrieveByKey(r, e.Key(), &readBack))
		require.Equal(t, e, readBack, "expected retrieved value to match written value")

		// Test write again should overwrite
		newEntity := Entity{ID: 42}
		require.NoError(t, withWriter(operation.Upsert(e.Key(), newEntity)))

		require.NoError(t, operation.RetrieveByKey(r, e.Key(), &readBack))
		require.Equal(t, newEntity, readBack, "expected overwritten value to be retrieved")

		// Test write should not overwrite a different key
		anotherEntity := Entity{ID: 84}
		require.NoError(t, withWriter(operation.Upsert(anotherEntity.Key(), anotherEntity)))

		var anotherReadBack Entity
		require.NoError(t, operation.RetrieveByKey(r, anotherEntity.Key(), &anotherReadBack))
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
		var err error
		exists, err = operation.KeyExists(r, e.Key())
		require.NoError(t, err)
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
			require.NoError(t, operation.RetrieveByKey(r, e.Key(), &readBack))
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
			err := operation.RetrieveByKey(r, e.Key(), &readBack)
			require.True(t, errors.Is(err, storage.ErrNotFound), "expected not found error for entity ID %d after removal", e.ID)
		}
	})
}

func TestBatchWriteArgumentCanBeModified(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		b := db.NewBatch()
		defer b.Close()

		k := []byte{0x01}
		v := []byte{0x02}

		// Insert k and v into batch.
		err := b.Writer().Set(k, v)
		require.NoError(t, err)

		// Modify k and v.
		k[0]++
		v[0]++

		// Commit batch.
		err = b.Commit()
		require.NoError(t, err)

		// Retrieve value with original key.
		retreivedValue, closer, err := db.Reader().Get([]byte{0x01})
		defer closer.Close()
		require.NoError(t, err)
		require.Equal(t, []byte{0x02}, retreivedValue)
	})
}

func TestBatchDeleteArgumentCanBeModified(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		{
			b := db.NewBatch()
			defer b.Close()

			k := []byte{0x01}
			v := []byte{0x02}

			// Insert k and v into batch.
			err := b.Writer().Set(k, v)
			require.NoError(t, err)

			// Commit batch.
			err = b.Commit()
			require.NoError(t, err)
		}
		{
			// Create batch to remove records.
			b := db.NewBatch()
			defer b.Close()

			k := []byte{0x01}

			// Delete record.
			err := b.Writer().Delete(k)
			require.NoError(t, err)

			// Modify k
			k[0]++

			// Commit batch.
			err = b.Commit()
			require.NoError(t, err)
		}
		{
			// Retrieve value with original key
			retreivedValue, closer, err := db.Reader().Get([]byte{0x01})
			defer closer.Close()
			require.ErrorIs(t, storage.ErrNotFound, err)
			require.Nil(t, retreivedValue)
		}
	})
}

func TestRemove(t *testing.T) {
	dbtest.RunWithStorages(t, func(t *testing.T, r storage.Reader, withWriter dbtest.WithWriter) {
		e := Entity{ID: 1337}

		var exists bool
		var err error
		exists, err = operation.KeyExists(r, e.Key())
		require.NoError(t, err)
		require.False(t, exists, "expected key to not exist")

		// Test delete nothing should return OK
		require.NoError(t, withWriter(operation.Remove(e.Key())))

		// Test write, delete, then read should return not found
		require.NoError(t, withWriter(operation.Upsert(e.Key(), e)))

		exists, err = operation.KeyExists(r, e.Key())
		require.NoError(t, err)
		require.True(t, exists, "expected key to exist")

		require.NoError(t, withWriter(operation.Remove(e.Key())))

		var item Entity
		err = operation.RetrieveByKey(r, e.Key(), &item)
		require.True(t, errors.Is(err, storage.ErrNotFound), "expected not found error after delete")
	})
}

func TestRemoveDiskUsage(t *testing.T) {
	const count = 10000

	opts := &pebble.Options{
		MemTableSize: 64 << 20, // required for rotating WAL
	}

	dbtest.RunWithPebbleDB(t, opts, func(t *testing.T, r storage.Reader, withWriter dbtest.WithWriter, dir string, db *pebble.DB) {
		prefix := []byte{1}
		endPrefix := []byte{2}
		getKey := func(c *flow.ChunkDataPack) []byte {
			return append(prefix, c.ChunkID[:]...)
		}

		items := make([]*flow.ChunkDataPack, count)
		for i := 0; i < count; i++ {
			chunkID := unittest.IdentifierFixture()
			chunkDataPack := unittest.ChunkDataPackFixture(chunkID)
			items[i] = chunkDataPack
		}

		// 1. Insert 10000 entities.
		require.NoError(t, withWriter(func(writer storage.Writer) error {
			for i := 0; i < count; i++ {
				if err := operation.Upsert(getKey(items[i]), items[i])(writer); err != nil {
					return err
				}
			}
			return nil
		}))

		// 2. Flush and compact to get a stable state.
		require.NoError(t, db.Flush())
		require.NoError(t, db.Compact(prefix, endPrefix, true))

		// 3. Get sizeBefore.
		sizeBefore := getFolderSize(t, dir)
		t.Logf("Size after initial write and compact: %d", sizeBefore)

		// 4. Remove all entities
		require.NoError(t, withWriter(func(writer storage.Writer) error {
			for i := 0; i < count; i++ {
				if err := operation.Remove(getKey(items[i]))(writer); err != nil {
					return err
				}
			}
			return nil
		}))

		// 5. Flush and compact again.
		require.NoError(t, db.Flush())
		require.NoError(t, db.Compact(prefix, endPrefix, true))

		// 6. Verify the disk usage is reduced.
		require.Eventually(t, func() bool {
			sizeAfter := getFolderSize(t, dir)
			t.Logf("Size after delete and compact: %d", sizeAfter)
			return sizeAfter < sizeBefore
		}, 30*time.Second, 200*time.Millisecond,
			"expected disk usage to be reduced after compaction. before: %d, after: %d",
			sizeBefore, getFolderSize(t, dir))
	})
}

// TestRemoveDiskUsageBadger verifies that Badger does not reclaim disk space after deletions,
// === RUN   TestRemoveDiskUsageBadger
//
//	writes_test.go:340: Badger - Size after initial write and GC: 7140099
//	writes_test.go:361: Badger - Size after delete and GC: 7640128
//	writes_test.go:362: Badger - Size difference: 500029 bytes (7.00%)
//
// --- PASS: TestRemoveDiskUsageBadger (0.13s)
func TestRemoveDiskUsageBadger(t *testing.T) {
	const count = 10000

	unittest.RunWithTempDir(t, func(dir string) {
		opts := badger.DefaultOptions(dir).
			WithKeepL0InMemory(true).
			WithLogger(nil)
		db, err := badger.Open(opts)
		require.NoError(t, err)
		defer func() {
			require.NoError(t, db.Close())
		}()

		withWriter := func(writing func(storage.Writer) error) error {
			return badgerimpl.ToDB(db).WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return writing(rw.Writer())
			})
		}

		prefix := []byte{1}
		getKey := func(c *flow.ChunkDataPack) []byte {
			return append(prefix, c.ChunkID[:]...)
		}

		items := make([]*flow.ChunkDataPack, count)
		for i := 0; i < count; i++ {
			chunkID := unittest.IdentifierFixture()
			chunkDataPack := unittest.ChunkDataPackFixture(chunkID)
			items[i] = chunkDataPack
		}

		// 1. Insert 10000 entities.
		require.NoError(t, withWriter(func(writer storage.Writer) error {
			for i := 0; i < count; i++ {
				if err := operation.Upsert(getKey(items[i]), items[i])(writer); err != nil {
					return err
				}
			}
			return nil
		}))

		// 2. Sync and run GC to get a stable state.
		require.NoError(t, db.Sync())
		err = db.RunValueLogGC(0.5)
		if err != nil && err != badger.ErrNoRewrite {
			require.NoError(t, err)
		}

		// 3. Get sizeBefore.
		sizeBefore := getFolderSize(t, dir)
		t.Logf("Badger - Size after initial write and GC: %d", sizeBefore)

		// 4. Remove all entities
		require.NoError(t, withWriter(func(writer storage.Writer) error {
			for i := 0; i < count; i++ {
				if err := operation.Remove(getKey(items[i]))(writer); err != nil {
					return err
				}
			}
			return nil
		}))

		// 5. Sync and run GC again.
		require.NoError(t, db.Sync())
		err = db.RunValueLogGC(0.5)
		if err != nil && err != badger.ErrNoRewrite {
			require.NoError(t, err)
		}

		// 6. Verify the disk usage is NOT reduced (Badger doesn't reclaim disk space).
		sizeAfter := getFolderSize(t, dir)
		t.Logf("Badger - Size after delete and GC: %d", sizeAfter)
		t.Logf("Badger - Size difference: %d bytes (%.2f%%)", sizeAfter-sizeBefore, float64(sizeAfter-sizeBefore)/float64(sizeBefore)*100)

		// Badger does not reclaim disk space after deletions, so sizeAfter should be >= sizeBefore
		// (or at least not significantly less)
		require.GreaterOrEqual(t, sizeAfter, sizeBefore,
			"Badger does not reclaim disk space after deletions. before: %d, after: %d",
			sizeBefore, sizeAfter)
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
				require.NoError(t, operation.RetrieveByKey(r, e.Key(), &readBack))
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
				err := operation.RetrieveByKey(r, e.Key(), &item)
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
			var err error
			exists, err = operation.KeyExists(r, key)
			require.NoError(t, err)
			t.Logf("key %x exists: %t", key, exists)

			deleted := includeStart <= i && i <= includeEnd

			// An item that was not deleted must exist
			require.Equal(t, !deleted, exists,
				"expected key %x to be %s", key, map[bool]string{true: "deleted", false: "not deleted"})
		}

		// Verify that after the removal, Traverse the removed prefix would return nothing
		removedKeys := make([]string, 0)
		err := operation.TraverseByPrefix(r, prefix, func(key []byte, getValue func(destVal any) error) (bail bool, err error) {
			removedKeys = append(removedKeys, fmt.Sprintf("%x", key))
			return false, nil
		}, storage.DefaultIteratorOptions())
		require.NoError(t, err)
		require.Len(t, removedKeys, 0, "expected no entries to be found when traversing the removed prefix")

		// Verify that after the removal, Iterate over all keys should only return keys outside the prefix range
		expected := [][]byte{
			{0x09, 0xff},
			{0x11, 0x00},
			{0x1A, 0xff},
		}

		actual := make([][]byte, 0)
		err = operation.IterateKeysByPrefixRange(r, []byte{keys[0][0]}, storage.PrefixUpperBound(keys[len(keys)-1]), func(key []byte) error {
			actual = append(actual, key)
			return nil
		})
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
			var err error
			exists, err = operation.KeyExists(r, key)
			require.NoError(t, err)
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
			var err error
			exists, err = operation.KeyExists(r, key)
			require.NoError(t, err)
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

// TestCompressionVerification verifies that data compression is actually working
// by comparing the on-disk size of databases with compression enabled vs disabled.
// The following test result shows an example output where Pebble compression works
// as expected, but Badger compression does not achieve the expected size reduction.
//
// === RUN   TestCompressionVerification
// === RUN   TestCompressionVerification/PebbleCompression
//
//	writes_test.go:785: Pebble - Size with compression: 10825774 bytes
//	writes_test.go:786: Pebble - Size without compression: 20560724 bytes
//	writes_test.go:787: Pebble - Compression ratio: 52.65%
//
// === RUN   TestCompressionVerification/BadgerCompression
//
//	writes_test.go:856: Badger - Size with compression: 10264989 bytes
//	writes_test.go:857: Badger - Size without compression: 10264989 bytes
//	writes_test.go:858: Badger - Compression ratio: 100.00%
//
// --- PASS: TestCompressionVerification (0.43s)
//
//	--- PASS: TestCompressionVerification/PebbleCompression (0.33s)
//	--- PASS: TestCompressionVerification/BadgerCompression (0.10s)
//
// PASS
// ok      github.com/onflow/flow-go/storage/operation     1.455s
func TestCompressionVerification(t *testing.T) {
	const (
		keyCount     = 1000
		valueSize    = 10 << 10 // 10 KB per value
		compressible = true     // Use highly compressible data (repeated patterns)
	)

	// Create highly compressible test data (repeated patterns compress well)
	createCompressibleValue := func(size int) []byte {
		pattern := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
		value := make([]byte, size)
		for i := 0; i < size; i++ {
			value[i] = pattern[i%len(pattern)]
		}
		return value
	}

	// Test with Pebble
	t.Run("PebbleCompression", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dir string) {
			// Create database with compression enabled (default is Snappy)
			optsWithCompression := &pebble.Options{
				FormatMajorVersion: pebble.FormatNewest,
				Levels:             make([]pebble.LevelOptions, 7),
			}
			for i := range optsWithCompression.Levels {
				optsWithCompression.Levels[i].Compression = func() pebble.Compression {
					return pebble.DefaultCompression // DefaultCompression uses Snappy
				}
				optsWithCompression.Levels[i].EnsureDefaults()
			}
			optsWithCompression.EnsureDefaults()

			dbWithCompression, err := pebble.Open(filepath.Join(dir, "compressed"), optsWithCompression)
			require.NoError(t, err)
			defer dbWithCompression.Close()

			// Create database with compression disabled
			optsNoCompression := &pebble.Options{
				FormatMajorVersion: pebble.FormatNewest,
				Levels:             make([]pebble.LevelOptions, 7),
			}
			for i := range optsNoCompression.Levels {
				optsNoCompression.Levels[i].Compression = func() pebble.Compression {
					return pebble.NoCompression
				}
				optsNoCompression.Levels[i].EnsureDefaults()
			}
			optsNoCompression.EnsureDefaults()

			dbNoCompression, err := pebble.Open(filepath.Join(dir, "uncompressed"), optsNoCompression)
			require.NoError(t, err)
			defer dbNoCompression.Close()

			// Write the same data to both databases
			testValue := createCompressibleValue(valueSize)
			writeData := func(db *pebble.DB) error {
				batch := db.NewBatch()
				defer batch.Close()

				for i := 0; i < keyCount; i++ {
					key := []byte(fmt.Sprintf("key-%d", i))
					if err := batch.Set(key, testValue, nil); err != nil {
						return err
					}
				}
				return batch.Commit(pebble.Sync)
			}

			require.NoError(t, writeData(dbWithCompression))
			require.NoError(t, writeData(dbNoCompression))

			// Flush and compact both databases to ensure data is written to disk
			require.NoError(t, dbWithCompression.Flush())
			require.NoError(t, dbNoCompression.Flush())

			require.NoError(t, dbWithCompression.Compact([]byte{0x00}, []byte{0xff}, true))
			require.NoError(t, dbNoCompression.Compact([]byte{0x00}, []byte{0xff}, true))

			// Measure on-disk sizes
			sizeWithCompression := getFolderSize(t, filepath.Join(dir, "compressed"))
			sizeNoCompression := getFolderSize(t, filepath.Join(dir, "uncompressed"))

			t.Logf("Pebble - Size with compression: %d bytes", sizeWithCompression)
			t.Logf("Pebble - Size without compression: %d bytes", sizeNoCompression)
			t.Logf("Pebble - Compression ratio: %.2f%%", float64(sizeWithCompression)/float64(sizeNoCompression)*100)

			// Verify that compressed database is significantly smaller
			// For highly compressible data, we expect at least 30% reduction
			compressionRatio := float64(sizeWithCompression) / float64(sizeNoCompression)
			require.Less(t, compressionRatio, 0.7, "compressed database should be at least 30%% smaller than uncompressed")
		})
	})

	// Test with Badger
	t.Run("BadgerCompression", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dir string) {
			// Create database with compression enabled (ZSTD)
			optsWithCompression := badger.DefaultOptions(filepath.Join(dir, "compressed")).
				WithCompression(options.ZSTD).
				WithZSTDCompressionLevel(3).
				WithKeepL0InMemory(true).
				WithLogger(nil)

			dbWithCompression, err := badger.Open(optsWithCompression)
			require.NoError(t, err)
			defer dbWithCompression.Close()

			// Create database with compression disabled
			optsNoCompression := badger.DefaultOptions(filepath.Join(dir, "uncompressed")).
				WithCompression(options.None).
				WithKeepL0InMemory(true).
				WithLogger(nil)

			dbNoCompression, err := badger.Open(optsNoCompression)
			require.NoError(t, err)
			defer dbNoCompression.Close()

			// Write the same data to both databases
			testValue := createCompressibleValue(valueSize)
			writeData := func(db *badger.DB) error {
				return db.Update(func(txn *badger.Txn) error {
					for i := 0; i < keyCount; i++ {
						key := []byte(fmt.Sprintf("key-%d", i))
						if err := txn.Set(key, testValue); err != nil {
							return err
						}
					}
					return nil
				})
			}

			require.NoError(t, writeData(dbWithCompression))
			require.NoError(t, writeData(dbNoCompression))

			// Force flush to ensure data is written to disk
			require.NoError(t, dbWithCompression.Sync())
			require.NoError(t, dbNoCompression.Sync())

			// Run value log GC to ensure data is compacted
			// This helps ensure we're measuring the actual on-disk size
			err = dbWithCompression.RunValueLogGC(0.5)
			if err != nil && err != badger.ErrNoRewrite {
				require.NoError(t, err)
			}
			err = dbNoCompression.RunValueLogGC(0.5)
			if err != nil && err != badger.ErrNoRewrite {
				require.NoError(t, err)
			}

			// Measure on-disk sizes
			sizeWithCompression := getFolderSize(t, filepath.Join(dir, "compressed"))
			sizeNoCompression := getFolderSize(t, filepath.Join(dir, "uncompressed"))

			t.Logf("Badger - Size with compression: %d bytes", sizeWithCompression)
			t.Logf("Badger - Size without compression: %d bytes", sizeNoCompression)
			t.Logf("Badger - Compression ratio: %.2f%%", float64(sizeWithCompression)/float64(sizeNoCompression)*100)

			// Verify that Badger does NOT compress data even when compression is enabled
			// The compression ratio should be close to 1.0 (no compression), proving that
			// Badger's compression setting has no effect
			compressionRatio := float64(sizeWithCompression) / float64(sizeNoCompression)
			require.GreaterOrEqual(t, compressionRatio, 0.95, "Badger does not compress data even when compression is enabled. compression ratio: %.2f%%", compressionRatio*100)
		})
	})
}

func TestBatchValue(t *testing.T) {
	const key = "key1"

	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		t.Run("no data", func(t *testing.T) {
			const expectedCallbackInvocationCount = 2
			callbackInvocationCount := 0

			err := db.WithReaderBatchWriter(func(b storage.ReaderBatchWriter) error {
				callbackFunc := func(error) {
					callbackInvocationCount++

					value, exists := b.ScopedValue(key)
					require.Nil(t, value)
					require.False(t, exists)
				}

				for range expectedCallbackInvocationCount {
					b.AddCallback(callbackFunc)
				}

				k := []byte{0x01}
				v := []byte{0x02}

				// Insert k and v into batch.
				err := b.Writer().Set(k, v)
				require.NoError(t, err)

				return nil
			})

			require.NoError(t, err)
			require.Equal(t, expectedCallbackInvocationCount, callbackInvocationCount)
		})

		t.Run("store data multiple times", func(t *testing.T) {
			const expectedCallbackInvocationCount = 2
			callbackInvocationCount := 0

			err := db.WithReaderBatchWriter(func(b storage.ReaderBatchWriter) error {
				b.SetScopedValue(key, []string{"value1", "value2"})

				b.SetScopedValue(key, []string{"value2", "value3"})

				callbackFunc := func(error) {
					callbackInvocationCount++

					data, exists := b.ScopedValue(key)
					require.Equal(t, []string{"value2", "value3"}, data.([]string))
					require.True(t, exists)
				}

				for range expectedCallbackInvocationCount {
					b.AddCallback(callbackFunc)
				}

				k := []byte{0x01}
				v := []byte{0x02}

				// Insert k and v into batch.
				err := b.Writer().Set(k, v)
				require.NoError(t, err)

				return nil
			})

			require.NoError(t, err)
			require.Equal(t, expectedCallbackInvocationCount, callbackInvocationCount)
		})

		t.Run("store and remove data", func(t *testing.T) {
			const expectedCallbackInvocationCount = 2
			callbackInvocationCount := 0

			err := db.WithReaderBatchWriter(func(b storage.ReaderBatchWriter) error {
				b.SetScopedValue(key, []string{"value1", "value2"})

				callbackFunc := func(error) {
					callbackInvocationCount++

					data, exists := b.ScopedValue(key)
					if callbackInvocationCount == 1 {
						require.Equal(t, []string{"value1", "value2"}, data.([]string))
						require.True(t, exists)

						b.SetScopedValue(key, nil)
					} else {
						require.Nil(t, data)
						require.False(t, exists)
					}
				}

				for range expectedCallbackInvocationCount {
					b.AddCallback(callbackFunc)
				}

				k := []byte{0x01}
				v := []byte{0x02}

				// Insert k and v into batch.
				err := b.Writer().Set(k, v)
				require.NoError(t, err)

				return nil
			})

			require.NoError(t, err)
			require.Equal(t, expectedCallbackInvocationCount, callbackInvocationCount)
		})
	})
}
