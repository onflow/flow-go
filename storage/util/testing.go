package util

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/module/metrics"
	storage "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func StorageLayer(t testing.TB, db *badger.DB) (*storage.Headers, *storage.Identities, *storage.Guarantees, *storage.Seals, *storage.Index, *storage.Payloads, *storage.Blocks) {
	metrics := metrics.NewNoopCollector()
	headers := storage.NewHeaders(metrics, db)
	identities := storage.NewIdentities(metrics, db)
	guarantees := storage.NewGuarantees(metrics, db)
	seals := storage.NewSeals(metrics, db)
	index := storage.NewIndex(metrics, db)
	payloads := storage.NewPayloads(index, identities, guarantees, seals)
	blocks := storage.NewBlocks(db, headers, payloads)
	return headers, identities, guarantees, seals, index, payloads, blocks
}

func RunWithStorageLayer(t testing.TB, f func(*badger.DB, *storage.Headers, *storage.Identities, *storage.Guarantees, *storage.Seals, *storage.Index, *storage.Payloads, *storage.Blocks)) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		headers, identities, guarantees, seals, index, payloads, blocks := StorageLayer(t, db)
		f(db, headers, identities, guarantees, seals, index, payloads, blocks)
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
