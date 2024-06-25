package pebble

import (
	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
)

func InitAll(metrics module.CacheMetrics, db *pebble.DB) *storage.All {
	headers := NewHeaders(metrics, db)
	guarantees := NewGuarantees(metrics, db, DefaultCacheSize)
	seals := NewSeals(metrics, db)
	index := NewIndex(metrics, db)
	results := NewExecutionResults(metrics, db)
	receipts := NewExecutionReceipts(metrics, db, results, DefaultCacheSize)
	payloads := NewPayloads(db, index, guarantees, seals, receipts, results)
	blocks := NewBlocks(db, headers, payloads)
	qcs := NewQuorumCertificates(metrics, db, DefaultCacheSize)
	setups := NewEpochSetups(metrics, db)
	epochCommits := NewEpochCommits(metrics, db)
	versionBeacons := NewVersionBeacons(db)

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
		QuorumCertificates: qcs,
		Setups:             setups,
		EpochCommits:       epochCommits,
		Statuses:           nil,
		VersionBeacons:     versionBeacons,
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
