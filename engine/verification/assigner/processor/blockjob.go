package processor

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// BlockJob implements the Job interface. It converts a Block into a Job to be used by job queue.
//
// In current architecture, BlockJob represents a finalized block enqueued to be processed by the BlockConsumer that implements the
// JobQueue interface.
type BlockJob struct {
	Block *flow.Block
}

// ID converts block id into job id, which guarantees uniqueness.
func (j BlockJob) ID() module.JobID {
	return jobID(j.Block.ID())
}

// jobID returns the corresponding unique job id of the BlockJob for this job.
func jobID(blockID flow.Identifier) module.JobID {
	return module.JobID(fmt.Sprintf("%v", blockID))
}

// block converts a block job into its corresponding block.
func toBlock(job module.Job) (*flow.Block, error) {
	blockJob, ok := job.(*BlockJob)
	if !ok {
		return nil, fmt.Errorf("could not assert job to block, job id: %x", job.ID())
	}
	return blockJob.Block, nil
}
