package store

import (
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
}

func InitAll(metrics module.CacheMetrics, db storage.DB) *All {
	headers := NewHeaders(metrics, db)
	guarantees := NewGuarantees(metrics, db, DefaultCacheSize)
	seals := NewSeals(metrics, db)
	index := NewIndex(metrics, db)
	results := NewExecutionResults(metrics, db)
	receipts := NewExecutionReceipts(metrics, db, results, DefaultCacheSize)
	payloads := NewPayloads(db, index, guarantees, seals, receipts, results)
	blocks := NewBlocks(db, headers, payloads)
	qcs := NewQuorumCertificates(metrics, db, DefaultCacheSize)

	return &All{
		Headers:            headers,
		Guarantees:         guarantees,
		Seals:              seals,
		Index:              index,
		Payloads:           payloads,
		Blocks:             blocks,
		QuorumCertificates: qcs,
		Results:            results,
		Receipts:           receipts,
	}
}
