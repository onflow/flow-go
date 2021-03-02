package assigner

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"

	"fmt"
)

// Indexer indexes the provided execution receipts. Each Receipt is indexed by the block the receipt pertains to as well as the
// identifier of the executor of receipt.
type Indexer interface {
	Index(Receipts []*flow.ExecutionReceipt) error
}

type ExecutionReceiptsIndexer struct {
	receipts storage.ExecutionReceipts
}

func NewExecutionReceiptsIndexer(receipts storage.ExecutionReceipts) *ExecutionReceiptsIndexer {
	return &ExecutionReceiptsIndexer{
		receipts: receipts,
	}
}

// Indexer indexes the provided execution receipts. Each Receipt is indexed by the block the receipt pertains to as well as the
// identifier of the executor of receipt.
func (e *ExecutionReceiptsIndexer) Index(receipts []*flow.ExecutionReceipt) error {
	for _, receipt := range receipts {
		err := e.receipts.IndexByExecutor(receipt)
		if err != nil {
			return fmt.Errorf("could not index receipt %v by executor ID %v: %w", receipt.ID(), receipt.ExecutorID, err)
		}
	}

	return nil
}
