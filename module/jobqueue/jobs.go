package jobqueue

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// JobID returns the corresponding unique job id of the BlockJob for this job.
func JobID(blockID flow.Identifier) module.JobID {
	return module.JobID(fmt.Sprintf("%v", blockID))
}

// BlockJob implements the Job interface. It converts a Block into a Job to be used by job queue.
//
// In current architecture, BlockJob represents a finalized block enqueued to be processed by the
// BlockConsumer that implements the JobQueue interface.
type BlockJob struct {
	Block *flow.Block
}

// ID converts block id into job id, which guarantees uniqueness.
func (j BlockJob) ID() module.JobID {
	return JobID(j.Block.ID())
}

// JobToBlock converts a block job into its corresponding block.
func JobToBlock(job module.Job) (*flow.Block, error) {
	blockJob, ok := job.(*BlockJob)
	if !ok {
		return nil, fmt.Errorf("could not assert job to block, job id: %x", job.ID())
	}
	return blockJob.Block, nil
}

// BlockToJob converts the block to a BlockJob.
func BlockToJob(block *flow.Block) *BlockJob {
	return &BlockJob{Block: block}
}

// BlockHeaderJob implements the Job interface. It converts a Block Header into a Job to be used by
// job queue.
//
// In current architecture, BlockHeaderJob represents a finalized block enqueued to be processed by
// a consumer that implements the JobQueue interface.
type BlockHeaderJob struct {
	Header *flow.Header
}

// ID converts block id into job id, which guarantees uniqueness.
func (j BlockHeaderJob) ID() module.JobID {
	return JobID(j.Header.ID())
}

// JobToBlockHeader converts a block job into its corresponding block header.
func JobToBlockHeader(job module.Job) (*flow.Header, error) {
	headerJob, ok := job.(*BlockHeaderJob)
	if !ok {
		return nil, fmt.Errorf("could not assert job to block header, job id: %x", job.ID())
	}
	return headerJob.Header, nil
}

// BlockHeaderToJob converts the block to a BlockHeaderJob.
func BlockHeaderToJob(header *flow.Header) *BlockHeaderJob {
	return &BlockHeaderJob{Header: header}
}
