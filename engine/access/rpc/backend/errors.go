package backend

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// InsufficientExecutionReceipts indicates that no execution receipt were found for a given block ID
type InsufficientExecutionReceipts struct {
	blockID      flow.Identifier
	receiptCount int
}

func (e InsufficientExecutionReceipts) Error() string {
	return fmt.Sprintf("insufficient execution receipts found (%d) for block ID: %s", e.receiptCount, e.blockID.String())
}
