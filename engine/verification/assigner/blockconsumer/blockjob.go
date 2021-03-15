package blockconsumer

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// BlockJob implements the Job interface. It converts a Block into a Job to be used by job queue.
type BlockJob struct {
	Block *flow.Block
}

// ID converts block id into job id, which guarantees uniqueness
func (j BlockJob) ID() module.JobID {
	return blockIDToJobID(j.Block.ID())
}

func blockIDToJobID(blockID flow.Identifier) module.JobID {
	return module.JobID(fmt.Sprintf("%v", blockID))
}

func jobToBlock(job module.Job) *flow.Block {
	blockJob, _ := job.(*BlockJob)
	return blockJob.Block
}
