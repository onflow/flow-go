package util

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/store"
)

func ExecutionStorageLayer(_ testing.TB, db storage.DB) *storage.Execution {
	metrics := metrics.NewNoopCollector()

	results := store.NewExecutionResults(metrics, db)
	receipts := store.NewExecutionReceipts(metrics, db, results, store.DefaultCacheSize)
	commits := store.NewCommits(metrics, db)
	transactionResults := store.NewTransactionResults(metrics, db, store.DefaultCacheSize)
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
