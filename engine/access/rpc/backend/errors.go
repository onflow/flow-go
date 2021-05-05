package backend

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// ExecutionReceiptNotFound indicates that no execution receipt were found for a given block ID
type ExecutionReceiptNotFound struct {
	blockID flow.Identifier
}

func (e ExecutionReceiptNotFound) Error() string {
	return fmt.Sprintf("no execution receipt found for block ID: %s", e.blockID.String())
}
