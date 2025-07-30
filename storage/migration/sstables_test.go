package migration

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/cockroachdb/pebble/v2/objstorage/objstorageprovider"
	"github.com/cockroachdb/pebble/v2/sstable"
	"github.com/cockroachdb/pebble/v2/vfs"
	"github.com/stretchr/testify/require"
)

func TestPebbleSSTableIngest(t *testing.T) {
	// Create a temporary directory for the Pebble DB
	dir, err := os.MkdirTemp("", "pebble-test")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	// Open Pebble DB
	db, err := pebble.Open(dir, &pebble.Options{
		FormatMajorVersion: pebble.FormatNewest,
	})
	require.NoError(t, err)
	defer db.Close()

	// Create an SSTable with a few key-values
	sstPath := filepath.Join(dir, "test.sst")
	file, err := vfs.Default.Create(sstPath, vfs.WriteCategoryUnspecified)
	require.NoError(t, err)
	writable := objstorageprovider.NewFileWritable(file)
	writer := sstable.NewWriter(writable, sstable.WriterOptions{
		TableFormat: sstable.TableFormatMax,
	})
	data := generateRandomKVData(500, 10, 50)

	// Sort the keys to ensure strictly increasing order
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		require.NoError(t, writer.Set([]byte(k), []byte(data[k])))
	}

	require.NoError(t, writer.Close())

	// Ingest the SSTable into Pebble DB
	require.NoError(t, db.Ingest(context.Background(), []string{sstPath}))

	// Verify the data exists
	for _, k := range keys {
		val, closer, err := db.Get([]byte(k))
		require.NoError(t, err, "expected key %s to exist", k)
		require.Equal(t, data[k], string(val))
		closer.Close()
	}
}
