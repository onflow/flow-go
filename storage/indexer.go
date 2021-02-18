package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

// Indexer indexes all execution receipts in a finalized container block based on
// their reference block ID and the executor ID.
type Indexer interface {
	IndexReceipts(blockID flow.Identifier)
}
