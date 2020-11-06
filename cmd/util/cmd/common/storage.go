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

type Storages struct {
	Headers            storage.Headers
	Guarantees         storage.Guarantees
	Seals              storage.Seals
	Index              storage.Index
	Payloads           storage.Payloads
	Blocks             storage.Blocks
	Setups             storage.EpochSetups
	EpochCommits       storage.EpochCommits
	Statuses           storage.EpochStatuses
	Results            storage.ExecutionResults
	Receipts           storage.ExecutionReceipts
	ChunkDataPacks     storage.ChunkDataPacks
	Commits            storage.Commits
	Transactions       storage.Transactions
	TransactionResults storage.TransactionResults
	Collections        storage.Collections
}

func InitStorages(db *badger.DB) *Storages {
	metrics := &metrics.NoopCollector{}

	headers := storagebadger.NewHeaders(metrics, db)
	guarantees := storagebadger.NewGuarantees(metrics, db)
	seals := storagebadger.NewSeals(metrics, db)
	index := storagebadger.NewIndex(metrics, db)
	payloads := storagebadger.NewPayloads(db, index, guarantees, seals)
	blocks := storagebadger.NewBlocks(db, headers, payloads)
	setups := storagebadger.NewEpochSetups(metrics, db)
	epochCommits := storagebadger.NewEpochCommits(metrics, db)
	statuses := storagebadger.NewEpochStatuses(metrics, db)
	results := storagebadger.NewExecutionResults(db)
	receipts := storagebadger.NewExecutionReceipts(db, results)
	chunkDataPacks := storagebadger.NewChunkDataPacks(db)
	commits := storagebadger.NewCommits(metrics, db)
	transactions := storagebadger.NewTransactions(metrics, db)
	transactionResults := storagebadger.NewTransactionResults(db)
	collections := storagebadger.NewCollections(db, transactions)

	return &Storages{
		Headers:            headers,
		Guarantees:         guarantees,
		Seals:              seals,
		Index:              index,
		Payloads:           payloads,
		Blocks:             blocks,
		Setups:             setups,
		EpochCommits:       epochCommits,
		Statuses:           statuses,
		Results:            results,
		Receipts:           receipts,
		ChunkDataPacks:     chunkDataPacks,
		Commits:            commits,
		Transactions:       transactions,
		TransactionResults: transactionResults,
		Collections:        collections,
	}
}
