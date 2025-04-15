package operation

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/require"
)

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
