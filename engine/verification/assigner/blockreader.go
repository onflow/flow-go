package assigner

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/state/protocol"
)

// FinalizedBlockReader provides an abstraction for consumers to read blocks
// as job
type FinalizedBlockReader struct {
	state protocol.State
}

// the job index would just be the finalized block height
func (r *FinalizedBlockReader) AtIndex(index int64) (module.Job, error) {
	var finalBlock *flow.Block
	return blockToJob(finalBlock), fmt.Errorf("to be implement")
}

// Head returns the last finalized height as job index
func (r *FinalizedBlockReader) Head() (int64, error) {
	return 0, fmt.Errorf("return the last finalized height")
}
