package jobs

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// JobID returns the corresponding unique job id for this job.
func JobID(id flow.Identifier) module.JobID {
	return module.JobID(fmt.Sprintf("%v", id))
}

// BlockJob implements the Job interface. It converts a Block into a Job to be used by job queue.
//
// In current architecture, BlockJob represents a finalized block enqueued to be processed by the
// BlockConsumer that implements the  JobQueue interface.
type BlockJob struct {
	Block *flow.Header
}

// ID converts block id into job id, which guarantees uniqueness.
func (j BlockJob) ID() module.JobID {
	return JobID(j.Block.ID())
}

// JobToBlock converts a block job into its corresponding block.
func JobToBlock(job module.Job) (*flow.Header, error) {
	blockJob, ok := job.(*BlockJob)
	if !ok {
		return nil, fmt.Errorf("could not assert job to block, job id: %x", job.ID())
	}
	return blockJob.Block, nil
}

// BlockToJob converts the block to a BlockJob.
func BlockToJob(block *flow.Header) *BlockJob {
	return &BlockJob{Block: block}
}

// NotifyJob implements the Job interface. It converts a BlockEntry into a Job to be used by job queue.
//
// In current architecture, BlockEntryJob represents a ExecutionData notification enqueued to be
// processed by the NotificationConsumer that implements the JobQueue interface.
type NotifyJob struct {
	Entry *BlockEntry
}

// ID converts block id into job id, which guarantees uniqueness.
func (j NotifyJob) ID() module.JobID {
	return JobID(j.Entry.BlockID)
}

// JobToBlockEntry converts a block entry job into its corresponding BlockEntry.
func JobToBlockEntry(job module.Job) (*BlockEntry, error) {
	blockJob, ok := job.(*NotifyJob)
	if !ok {
		return nil, fmt.Errorf("could not assert job to block, job id: %x", job.ID())
	}
	return blockJob.Entry, nil
}

// BlockEntryToJob converts the BlockEntry to a NotifyJob.
func BlockEntryToJob(entry *BlockEntry) *NotifyJob {
	return &NotifyJob{Entry: entry}
}
