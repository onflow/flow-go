package status

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/state_synchronization"
)

// BlockEntry represents a block that's tracked by the ExecutionDataRequester
type BlockEntry struct {
	BlockID       flow.Identifier
	Height        uint64
	ExecutionData *state_synchronization.ExecutionData
}
