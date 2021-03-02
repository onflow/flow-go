package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

// Indexer indexes the provided execution receipts. Each Receipt is indexed by the block the receipt pertains to.
type Indexer interface {
	IndexReceipts(Receipts []*ExecutionReceipt) error
}
