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
	qcs := NewQuorumCertificates(metrics, db, DefaultCacheSize)
	setups := NewEpochSetups(metrics, db)
	epochCommits := NewEpochCommits(metrics, db)
	epochProtocolStateEntries := NewEpochProtocolStateEntries(metrics, setups, epochCommits, db,
		DefaultEpochProtocolStateCacheSize, DefaultProtocolStateIndexCacheSize)
	protocolKVStore := NewProtocolKVStore(metrics, db, DefaultProtocolKVStoreCacheSize, DefaultProtocolKVStoreByBlockIDCacheSize)
	versionBeacons := NewVersionBeacons(db)

	commits := NewCommits(metrics, db)
	transactions := NewTransactions(metrics, db)
	transactionResults := NewTransactionResults(metrics, db, 10000)
	collections := NewCollections(db, transactions)
	events := NewEvents(metrics, db)

	return &storage.All{
		Headers:                   headers,
		Guarantees:                guarantees,
		Seals:                     seals,
		Index:                     index,
		Payloads:                  payloads,
		Blocks:                    blocks,
		QuorumCertificates:        qcs,
		Setups:                    setups,
		EpochCommits:              epochCommits,
		EpochProtocolStateEntries: epochProtocolStateEntries,
		ProtocolKVStore:           protocolKVStore,
		VersionBeacons:            versionBeacons,
		Results:                   results,
		Receipts:                  receipts,
		Commits:                   commits,
		Transactions:              transactions,
		TransactionResults:        transactionResults,
		Collections:               collections,
		Events:                    events,
	}
}
