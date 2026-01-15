package store

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
)

type All struct {
	Headers            *Headers
	Guarantees         *Guarantees
	Seals              *Seals
	Index              *Index
	Payloads           *Payloads
	Blocks             *Blocks
	QuorumCertificates *QuorumCertificates
	Results            *ExecutionResults
	Receipts           *ExecutionReceipts
	Commits            *Commits

	EpochSetups               *EpochSetups
	EpochCommits              *EpochCommits
	EpochProtocolStateEntries *EpochProtocolStateEntries
	ProtocolKVStore           *ProtocolKVStore
	VersionBeacons            *VersionBeacons

	Transactions *Transactions
	Collections  *Collections
}

// InitAll initializes the common storage abstractions used by all node roles (with default cache sizes
// suitable for mainnet). The chain ID indicates which Flow network the node is operating on and references
// the ID of the main consensus (not the chains built by collector clusters)
// No errors are expected during normal operations.
func InitAll(metrics module.CacheMetrics, db storage.DB, chainID flow.ChainID) (*All, error) {
	headers, err := NewHeaders(metrics, db, chainID)
	if err != nil {
		return nil, fmt.Errorf("instantiating header storage abstraction failed: %w", err)
	}
	guarantees := NewGuarantees(metrics, db, DefaultCacheSize, DefaultCacheSize)
	seals := NewSeals(metrics, db)
	index := NewIndex(metrics, db)
	results := NewExecutionResults(metrics, db)
	receipts := NewExecutionReceipts(metrics, db, results, DefaultCacheSize)
	payloads := NewPayloads(db, index, guarantees, seals, receipts, results)
	blocks := NewBlocks(db, headers, payloads)
	qcs := NewQuorumCertificates(metrics, db, DefaultCacheSize)
	commits := NewCommits(metrics, db)

	setups := NewEpochSetups(metrics, db)
	epochCommits := NewEpochCommits(metrics, db)
	epochProtocolStateEntries := NewEpochProtocolStateEntries(metrics, setups, epochCommits, db,
		DefaultEpochProtocolStateCacheSize, DefaultProtocolStateIndexCacheSize)
	protocolKVStore := NewProtocolKVStore(metrics, db, DefaultProtocolKVStoreCacheSize, DefaultProtocolKVStoreByBlockIDCacheSize)
	versionBeacons := NewVersionBeacons(db)

	transactions := NewTransactions(metrics, db)
	collections := NewCollections(db, transactions)

	return &All{
		Headers:                   headers,
		Guarantees:                guarantees,
		Seals:                     seals,
		Index:                     index,
		Payloads:                  payloads,
		Blocks:                    blocks,
		QuorumCertificates:        qcs,
		Results:                   results,
		Receipts:                  receipts,
		Commits:                   commits,
		EpochCommits:              epochCommits,
		EpochSetups:               setups,
		EpochProtocolStateEntries: epochProtocolStateEntries,
		ProtocolKVStore:           protocolKVStore,
		VersionBeacons:            versionBeacons,

		Transactions: transactions,
		Collections:  collections,
	}, nil
}
