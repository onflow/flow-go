package extended

import (
	"fmt"
	"io"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/indexes"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	pstorage "github.com/onflow/flow-go/storage/pebble"
)

// BootstrapExtendedIndexes bootstraps the extended index database, and instantiates the extended indexer.
//
// No error returns are expected during normal operation.
func BootstrapExtendedIndexes(
	log zerolog.Logger,
	state protocol.State,
	blocks storage.Blocks,
	collections storage.Collections,
	events storage.Events,
	lightTransactionResults storage.LightTransactionResults,
	lockManager storage.LockManager,
	dbPath string,
	backfillDelay time.Duration,
) (*ExtendedIndexer, io.Closer, error) {
	indexerDB, err := pstorage.SafeOpen(
		log.With().Str("pebbledb", "indexer").Logger(),
		dbPath,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("could not open indexer db: %w", err)
	}

	chainID := state.Params().ChainID()

	indexerStorageDB := pebbleimpl.ToDB(indexerDB)
	accountTxStore, err := indexes.NewAccountTransactionsBootstrapper(
		indexerStorageDB,
		state.Params().SealedRoot().Height,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("could not create account transactions index: %w", err)
	}

	extendedIndexers := []Indexer{
		NewAccountTransactions(log, accountTxStore, chainID, lockManager),
	}

	extendedIndexer, err := NewExtendedIndexer(
		log,
		metrics.NewExtendedIndexingCollector(),
		indexerStorageDB,
		lockManager,
		state,
		blocks,
		collections,
		events,
		lightTransactionResults,
		extendedIndexers,
		chainID,
		backfillDelay,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("could not create extended indexer: %w", err)
	}

	return extendedIndexer, indexerDB, nil
}
