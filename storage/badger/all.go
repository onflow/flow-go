package badger

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
)

func InitAll(metrics module.CacheMetrics, db *badger.DB) *storage.All {
	headers := NewHeaders(metrics, db)
	guarantees := NewGuarantees(metrics, db)
	seals := NewSeals(metrics, db)
	index := NewIndex(metrics, db)
	payloads := NewPayloads(db, index, guarantees, seals)
	blocks := NewBlocks(db, headers, payloads)
	setups := NewEpochSetups(metrics, db)
	epochCommits := NewEpochCommits(metrics, db)
	statuses := NewEpochStatuses(metrics, db)
	results := NewExecutionResults(db)
	receipts := NewExecutionReceipts(db, results)
	chunkDataPacks := NewChunkDataPacks(db)
	commits := NewCommits(metrics, db)
	transactions := NewTransactions(metrics, db)
	transactionResults := NewTransactionResults(db)
	collections := NewCollections(db, transactions)
	events := NewEvents(db)

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
