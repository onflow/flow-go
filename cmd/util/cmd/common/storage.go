package common

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	storagebadger "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/operation/badgerimpl"
	"github.com/onflow/flow-go/storage/store"
)

func InitStorage(datadir string) *badger.DB {
	return InitStorageWithTruncate(datadir, false)
}

func InitStorageWithTruncate(datadir string, truncate bool) *badger.DB {
	opts := badger.
		DefaultOptions(datadir).
		WithKeepL0InMemory(true).
		WithLogger(nil).
		WithTruncate(truncate)

	db, err := badger.Open(opts)
	if err != nil {
		log.Fatal().Err(err).Msg("could not open key-value store")
	}

	// in order to void long iterations with big keys when initializing with an
	// already populated database, we bootstrap the initial maximum key size
	// upon starting
	err = operation.RetryOnConflict(db.Update, func(tx *badger.Txn) error {
		return operation.InitMax(tx)
	})
	if err != nil {
		log.Fatal().Err(err).Msg("could not initialize max tracker")
	}

	return db
}

func InitStorages(db *badger.DB) *storage.All {
	metrics := &metrics.NoopCollector{}

	return storagebadger.InitAll(metrics, db)
}

func InitExecutionStorages(bdb *badger.DB) *storage.Execution {
	metrics := &metrics.NoopCollector{}

	db := badgerimpl.ToDB(bdb)

	results := store.NewExecutionResults(metrics, db)
	receipts := store.NewExecutionReceipts(metrics, db, results, storagebadger.DefaultCacheSize)
	commits := store.NewCommits(metrics, db)
	transactionResults := store.NewTransactionResults(metrics, db, storagebadger.DefaultCacheSize)
	events := store.NewEvents(metrics, db)
	return &storage.Execution{
		Results:            results,
		Receipts:           receipts,
		Commits:            commits,
		TransactionResults: transactionResults,
		Events:             events,
	}
}
