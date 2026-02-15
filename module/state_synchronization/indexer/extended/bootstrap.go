package extended

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/indexes"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	pstorage "github.com/onflow/flow-go/storage/pebble"
)

type Storage struct {
	DB                              storage.DB
	AccountTransactionsBootstrapper storage.AccountTransactionsBootstrapper
}

// OpenExtendedIndexDB opens the pebble database for extended indexes and creates the account
// transactions bootstrapper store. This must run synchronously during node initialization so
// that the store is available for consumers (e.g. the ExtendedBackend) before async components
// start.
//
// No error returns are expected during normal operation.
func OpenExtendedIndexDB(
	log zerolog.Logger,
	dbPath string,
	sealedRootHeight uint64,
) (Storage, error) {
	indexerDB, err := pstorage.SafeOpen(
		log.With().Str("pebbledb", "indexer").Logger(),
		dbPath,
	)
	if err != nil {
		return Storage{}, fmt.Errorf("could not open indexer db: %w", err)
	}

	indexerStorageDB := pebbleimpl.ToDB(indexerDB)
	accountTxStore, err := indexes.NewAccountTransactionsBootstrapper(
		indexerStorageDB,
		sealedRootHeight,
	)
	if err != nil {
		if closeErr := indexerDB.Close(); closeErr != nil {
			log.Error().Err(closeErr).Msg("error closing indexer db")
		}
		return Storage{}, fmt.Errorf("could not create account transactions index: %w", err)
	}

	return Storage{
		DB:                              indexerStorageDB,
		AccountTransactionsBootstrapper: accountTxStore,
	}, nil
}
