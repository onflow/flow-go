package backend

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// SnapshotPhaseMismatchError indicates that a valid sealing segment cannot be build for a snapshot because
// the snapshot requested spans either an epoch transition or phase transition.
var SnapshotPhaseMismatchError = errors.New("snapshot does not contain a valid sealing segment")

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
