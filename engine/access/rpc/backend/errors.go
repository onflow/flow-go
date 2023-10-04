package backend

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// ErrDataNotAvailable indicates that the data for a given block was not available
//
// This generally indicates a request was made for execution data at a block height that was not
// not locally indexed
var ErrDataNotAvailable = errors.New("data for block is not available")

// InsufficientExecutionReceipts indicates that no execution receipt were found for a given block ID
type InsufficientExecutionReceipts struct {
	blockID      flow.Identifier
	receiptCount int
}

func (e InsufficientExecutionReceipts) Error() string {
	return fmt.Sprintf("insufficient execution receipts found (%d) for block ID: %s", e.receiptCount, e.blockID.String())
}

func IsInsufficientExecutionReceipts(err error) bool {
	var errInsufficientExecutionReceipts InsufficientExecutionReceipts
	return errors.As(err, &errInsufficientExecutionReceipts)
}
