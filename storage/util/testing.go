package util

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	storage "github.com/onflow/flow-go/storage"

	"github.com/onflow/flow-go/module/metrics"
	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/utils/unittest"
)

func StorageLayer(t testing.TB, db *badger.DB) *storage.All {
	metrics := metrics.NewNoopCollector()
	all := bstorage.InitAll(metrics, db)
	return all
}

func RunWithStorageLayer(t testing.TB, f func(*badger.DB, *storage.All)) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		all := StorageLayer(t, db)
		f(db, all)
	})
}

func CreateFiles(t *testing.T, dir string, names ...string) {
	for _, name := range names {
		file, err := os.Create(filepath.Join(dir, name))
		require.NoError(t, err)
		err = file.Close()
		require.NoError(t, err)
	}
}
