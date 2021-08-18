package badger

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
)

func InitAll(metrics module.CacheMetrics, db *badger.DB) *storage.All {
	headers := NewHeaders(metrics, db)
	guarantees := NewGuarantees(metrics, db, DefaultCacheSize)
	seals := NewSeals(metrics, db)
	index := NewIndex(metrics, db)
	results := NewExecutionResults(metrics, db)
	receipts := NewExecutionReceipts(metrics, db, results, DefaultCacheSize)
	payloads := NewPayloads(db, index, guarantees, seals, receipts, results)
	blocks := NewBlocks(db, headers, payloads)
	setups := NewEpochSetups(metrics, db)
	epochCommits := NewEpochCommits(metrics, db)
	statuses := NewEpochStatuses(metrics, db)

	commits := NewCommits(metrics, db)
	transactions := NewTransactions(metrics, db)
	transactionResults := NewTransactionResults(metrics, db, 10000)
	collections := NewCollections(db, transactions)
	events := NewEvents(metrics, db)
	chunkDataPacks := NewChunkDataPacks(metrics, db, collections, 1000)

	return &storage.All{
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
		Events:             events,
	}
}
