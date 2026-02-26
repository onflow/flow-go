package bootstrap

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/state_synchronization/indexer/extended"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/indexes"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	pstorage "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/utils"
)

type Storage struct {
	DB                                    storage.DB
	AccountTransactionsBootstrapper       storage.AccountTransactionsBootstrapper
	FungibleTokenTransfersBootstrapper    storage.FungibleTokenTransfersBootstrapper
	NonFungibleTokenTransfersBootstrapper storage.NonFungibleTokenTransfersBootstrapper
	ScheduledTransactionsBootstrapper     storage.ScheduledTransactionsIndexBootstrapper
	ContractDeploymentsBootstrapper       storage.ContractDeploymentsIndexBootstrapper
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
	accountTxStore, err := indexes.NewAccountTransactionsBootstrapper(indexerStorageDB, sealedRootHeight)
	if err != nil {
		if closeErr := indexerDB.Close(); closeErr != nil {
			log.Error().Err(closeErr).Msg("error closing indexer db")
		}
		return Storage{}, fmt.Errorf("could not create account transactions index: %w", err)
	}

	ftStore, err := indexes.NewFungibleTokenTransfersBootstrapper(indexerStorageDB, sealedRootHeight)
	if err != nil {
		if closeErr := indexerDB.Close(); closeErr != nil {
			log.Error().Err(closeErr).Msg("error closing indexer db")
		}
		return Storage{}, fmt.Errorf("could not create fungible token transfers index: %w", err)
	}

	nftStore, err := indexes.NewNonFungibleTokenTransfersBootstrapper(indexerStorageDB, sealedRootHeight)
	if err != nil {
		if closeErr := indexerDB.Close(); closeErr != nil {
			log.Error().Err(closeErr).Msg("error closing indexer db")
		}
		return Storage{}, fmt.Errorf("could not create non-fungible token transfers index: %w", err)
	}

	scheduledTxStore, err := indexes.NewScheduledTransactionsBootstrapper(indexerStorageDB, sealedRootHeight)
	if err != nil {
		if closeErr := indexerDB.Close(); closeErr != nil {
			log.Error().Err(closeErr).Msg("error closing indexer db")
		}
		return Storage{}, fmt.Errorf("could not create scheduled transactions index: %w", err)
	}

	contractDeploymentsStore, err := indexes.NewContractDeploymentsBootstrapper(indexerStorageDB, sealedRootHeight)
	if err != nil {
		if closeErr := indexerDB.Close(); closeErr != nil {
			log.Error().Err(closeErr).Msg("error closing indexer db")
		}
		return Storage{}, fmt.Errorf("could not create contract deployments index: %w", err)
	}

	return Storage{
		DB:                                    indexerStorageDB,
		AccountTransactionsBootstrapper:       accountTxStore,
		FungibleTokenTransfersBootstrapper:    ftStore,
		NonFungibleTokenTransfersBootstrapper: nftStore,
		ScheduledTransactionsBootstrapper:     scheduledTxStore,
		ContractDeploymentsBootstrapper:       contractDeploymentsStore,
	}, nil
}

func BootstrapIndexers(
	log zerolog.Logger,
	chainID flow.ChainID,
	extendedStorage Storage,
	lockManager storage.LockManager,
	state protocol.State,
	index storage.Index,
	headers storage.Headers,
	guarantees storage.Guarantees,
	collections storage.Collections,
	events storage.Events,
	results storage.LightTransactionResults,
	scriptExecutor execution.ScriptExecutor,
	backfillDelay time.Duration,
) (*extended.ExtendedIndexer, error) {
	accountTransactions, err := extended.NewAccountTransactions(
		log,
		extendedStorage.AccountTransactionsBootstrapper,
		chainID,
		utils.NotNil(lockManager),
	)
	if err != nil {
		return nil, fmt.Errorf("could not create account transactions indexer: %w", err)
	}

	extendedMetrics := metrics.NewExtendedIndexingCollector()

	ftTransfers := extended.NewFungibleTokenTransfers(
		log,
		chainID,
		extendedStorage.FungibleTokenTransfersBootstrapper,
		extendedMetrics,
	)

	nftTransfers := extended.NewNonFungibleTokenTransfers(
		log,
		chainID,
		extendedStorage.NonFungibleTokenTransfersBootstrapper,
		extendedMetrics,
	)

	scheduledTransactions := extended.NewScheduledTransactions(
		log,
		extendedStorage.ScheduledTransactionsBootstrapper,
		scriptExecutor,
		extendedMetrics,
		chainID,
	)

	contracts := extended.NewContracts(
		log,
		chainID.Chain(),
		extendedStorage.ContractDeploymentsBootstrapper,
		scriptExecutor,
	)

	extendedIndexers := []extended.Indexer{
		accountTransactions,
		ftTransfers,
		nftTransfers,
		scheduledTransactions,
		contracts,
	}

	extendedIndexer, err := extended.NewExtendedIndexer(
		log,
		extendedMetrics,
		extendedStorage.DB,
		lockManager,
		state,
		index,
		headers,
		guarantees,
		collections,
		events,
		results,
		extendedIndexers,
		chainID,
		backfillDelay,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create extended indexer: %w", err)
	}

	return extendedIndexer, nil
}
