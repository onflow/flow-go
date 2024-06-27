package util

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	bstorage "github.com/onflow/flow-go/storage/badger"
)

func StorageLayer(_ testing.TB, db *badger.DB) *storage.All {
	metrics := metrics.NewNoopCollector()
	all := bstorage.InitAll(metrics, db)
	return all
}

func CreateFiles(t *testing.T, dir string, names ...string) {
	for _, name := range names {
		file, err := os.Create(filepath.Join(dir, name))
		require.NoError(t, err)
		err = file.Close()
		require.NoError(t, err)
	}
}
