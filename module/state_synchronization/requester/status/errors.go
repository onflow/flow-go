package status

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

type RequesterHaltedError struct {
	ExecutionDataID flow.Identifier
	BlockID         flow.Identifier
	Height          uint64
	Err             error
}

func (e *RequesterHaltedError) Error() string {
	return fmt.Sprintf("execution data requester halted on execution data %v, block %v, height %d: %s",
		e.ExecutionDataID, e.BlockID, e.Height, e.Err)
}
