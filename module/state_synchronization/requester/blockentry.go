package requester

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/state_synchronization"
)

type BlockEntry struct {
	BlockID       flow.Identifier
	Height        uint64
	ExecutionData *state_synchronization.ExecutionData

	index int // used by heap
}
