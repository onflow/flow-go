package operation_test

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/cockroachdb/pebble"
)

// TestDiskSpaceReclamation validates that disk space is reclaimed after deleting data and compacting the database.
func TestDiskSpaceReclamation(t *testing.T) {
	// Setup: Create a temporary Pebble database.
	dbPath := "./pebble-test"
	absPath, err := filepath.Abs(dbPath)
	if err != nil {
		log.Fatalf("Error getting absolute path: %v", err)
	}
	t.Logf("path: %v", absPath)

	opts := &pebble.Options{}
	db, err := pebble.Open(dbPath, opts)
	if err != nil {
		t.Fatalf("Failed to open Pebble DB: %v", err)
	}
	defer cleanup(db, dbPath)

	// Step 1: Insert a large number of key-value pairs.
	t.Log("Inserting data...")
	for i := 0; i < 100000; i++ {
		key := []byte(fmt.Sprintf("key-%d", i))
		value := []byte("This is some test data.")
		if err := db.Set(key, value, pebble.Sync); err != nil {
			t.Fatalf("Failed to set key: %v", err)
		}
	}

	// Step 2: Measure disk usage after insertion.
	beforeDeleteSize := getDirSize(dbPath)
	t.Logf("Disk usage after insertion: %d bytes", beforeDeleteSize)

	// Step 3: Delete the inserted keys.
	t.Log("Deleting data...")
	for i := 0; i < 100000; i++ {
		key := []byte(fmt.Sprintf("key-%d", i))
		if err := db.Delete(key, pebble.Sync); err != nil {
			t.Fatalf("Failed to delete key: %v", err)
		}
	}

	// Step 4: Trigger manual compaction to reclaim space.
	t.Log("Triggering compaction...")
	if err := db.Compact([]byte("key"), []byte("sss"), true); err != nil {
		t.Fatalf("Failed to compact database: %v", err)
	}

	// Step 5: Measure disk usage after deletion and compaction.
	afterDeleteSize := getDirSize(dbPath)
	t.Logf("Disk usage after deletion and compaction: %d bytes", afterDeleteSize)

	// Step 6: Validate that disk space was reclaimed.
	if afterDeleteSize >= beforeDeleteSize {
		t.Errorf("Disk space was not reclaimed: before=%d bytes, after=%d bytes", beforeDeleteSize, afterDeleteSize)
	} else {
		t.Log("Disk space was successfully reclaimed.")
	}
}

// cleanup closes the database and removes the temporary directory.
func cleanup(db *pebble.DB, path string) {
	_ = db.Close()
	_ = os.RemoveAll(path)
}

// getDirSize calculates the size of a directory in bytes.
func getDirSize(path string) int64 {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		size += info.Size()
		return nil
	})
	if err != nil {
		log.Fatalf("Failed to calculate directory size: %v", err)
	}
	return size
}
