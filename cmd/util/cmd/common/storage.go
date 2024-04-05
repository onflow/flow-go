package common

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	storagebadger "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/badger/operation"
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

	err = operation.SyncCompressionFlag(db)
	if err != nil {
		log.Fatal().Err(err).Msg("could not sync compression flag")
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
