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
	"github.com/onflow/flow-go/storage/operation/badgerimpl"
	"github.com/onflow/flow-go/storage/store"
)

func StorageLayer(_ testing.TB, db *badger.DB) *storage.All {
	metrics := metrics.NewNoopCollector()
	all := bstorage.InitAll(metrics, db)
	return all
}

func ExecutionStorageLayer(_ testing.TB, bdb *badger.DB) *storage.Execution {
	metrics := metrics.NewNoopCollector()

	db := badgerimpl.ToDB(bdb)

	results := store.NewExecutionResults(metrics, db)
	receipts := store.NewExecutionReceipts(metrics, db, results, bstorage.DefaultCacheSize)
	commits := store.NewCommits(metrics, db)
	transactionResults := store.NewTransactionResults(metrics, db, bstorage.DefaultCacheSize)
	events := store.NewEvents(metrics, db)
	return &storage.Execution{
		Results:            results,
		Receipts:           receipts,
		Commits:            commits,
		TransactionResults: transactionResults,
		Events:             events,
	}
}

func CreateFiles(t *testing.T, dir string, names ...string) {
	for _, name := range names {
		file, err := os.Create(filepath.Join(dir, name))
		require.NoError(t, err)
		err = file.Close()
		require.NoError(t, err)
	}
}
