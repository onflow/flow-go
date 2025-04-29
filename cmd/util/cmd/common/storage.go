package common

import (
	"fmt"

	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	storagebadger "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/operation/badgerimpl"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	pebblestorage "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/storage/store"
)

// DBDirs is a struct that holds the datadir and pebble-dir
// this struct help prevents mistakes from passing the wrong dir, such as pass pebbledir as datadir
type DBDirs struct {
	Datadir   string
	Pebbledir string
}

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

// WithStorage runs the given function with the storage dependending on the flags
// only one flag (datadir / pebble-dir) is allowed to be set
func WithStorage(dirs DBDirs, f func(storage.DB) error) error {
	if dirs.Pebbledir != "" {
		if dirs.Datadir != "" {
			log.Warn().Msg("both --datadir and --pebble-dir are set, using --pebble-dir")
		}

		db, err := pebblestorage.MustOpenDefaultPebbleDB(log.Logger, dirs.Pebbledir)
		if err != nil {
			return err
		}

		defer db.Close()

		log.Info().Msgf("using pebble db at %s", dirs.Pebbledir)
		return f(pebbleimpl.ToDB(db))
	}

	if dirs.Datadir != "" {
		db := InitStorage(dirs.Datadir)
		defer db.Close()

		log.Info().Msgf("using badger db at %s", dirs.Datadir)
		return f(badgerimpl.ToDB(db))
	}

	return fmt.Errorf("must specify either --datadir or --pebble-dir")
}

// InitBadgerAndPebble initializes the badger and pebble storages
func InitBadgerAndPebble(dirs DBDirs) (bdb *badger.DB, pdb *pebble.DB, err error) {
	if dirs.Datadir == "" {
		return nil, nil, fmt.Errorf("must specify --datadir")
	}

	if dirs.Pebbledir == "" {
		return nil, nil, fmt.Errorf("must specify --pebble-dir")
	}

	pdb, err = pebblestorage.MustOpenDefaultPebbleDB(
		log.Logger.With().Str("pebbledb", "protocol").Logger(), dirs.Pebbledir)
	if err != nil {
		return nil, nil, err
	}

	bdb = InitStorage(dirs.Datadir)

	return bdb, pdb, nil
}

// WithBadgerAndPebble runs the given function with the badger and pebble storages
// it ensures that the storages are closed after the function is done
func WithBadgerAndPebble(dirs DBDirs, f func(*badger.DB, *pebble.DB) error) error {
	bdb, pdb, err := InitBadgerAndPebble(dirs)
	if err != nil {
		return err
	}

	defer bdb.Close()
	defer pdb.Close()

	return f(bdb, pdb)
}
