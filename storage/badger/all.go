package badger

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/badgerimpl"
	"github.com/onflow/flow-go/storage/store"
)

// TODO: deprecated, keep it until all modules are migrated to the new storage
func InitAllBadger(metrics module.CacheMetrics, db *badger.DB) *storage.All {
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
	versionBeacons := store.NewVersionBeacons(badgerimpl.ToDB(db))

	transactions := NewTransactions(metrics, db)
	collections := NewCollections(db, transactions)

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
		Transactions:              transactions,
		Collections:               collections,
	}
}

func InitAll(metrics module.CacheMetrics, db *badger.DB) *storage.All {
	sdb := badgerimpl.ToDB(db)
	headers := store.NewHeaders(metrics, sdb)
	guarantees := store.NewGuarantees(metrics, sdb, DefaultCacheSize)
	seals := store.NewSeals(metrics, sdb)
	index := store.NewIndex(metrics, sdb)
	results := store.NewExecutionResults(metrics, sdb)
	receipts := store.NewExecutionReceipts(metrics, sdb, results, DefaultCacheSize)
	payloads := store.NewPayloads(sdb, index, guarantees, seals, receipts, results)
	blocks := store.NewBlocks(sdb, headers, payloads)
	qcs := store.NewQuorumCertificates(metrics, sdb, DefaultCacheSize)
	setups := store.NewEpochSetups(metrics, sdb)
	epochCommits := store.NewEpochCommits(metrics, sdb)
	epochProtocolStateEntries := store.NewEpochProtocolStateEntries(metrics, setups, epochCommits, sdb,
		DefaultEpochProtocolStateCacheSize, DefaultProtocolStateIndexCacheSize)
	protocolKVStore := store.NewProtocolKVStore(metrics, sdb, DefaultProtocolKVStoreCacheSize, DefaultProtocolKVStoreByBlockIDCacheSize)
	versionBeacons := store.NewVersionBeacons(sdb)

	transactions := NewTransactions(metrics, db)
	collections := NewCollections(db, transactions)

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
		Transactions:              transactions,
		Collections:               collections,
	}
}
