package jobs

import (
	"fmt"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/jobqueue"
)

// BlockEntryJob implements the Job interface. It converts a BlockEntry into a Job to be used by job queue.
//
// In current architecture, BlockEntryJob represents a ExecutionData notification enqueued to be
// processed by the NotificationConsumer that implements the JobQueue interface.
type BlockEntryJob struct {
	Entry *BlockEntry
}

// ID converts block id into job id, which guarantees uniqueness.
func (j BlockEntryJob) ID() module.JobID {
	return jobqueue.JobID(j.Entry.BlockID)
}

// JobToBlockEntry converts a block entry job into its corresponding BlockEntry.
func JobToBlockEntry(job module.Job) (*BlockEntry, error) {
	blockJob, ok := job.(*BlockEntryJob)
	if !ok {
		return nil, fmt.Errorf("could not convert job to block entry, job id: %x", job.ID())
	}
	return blockJob.Entry, nil
}

// BlockEntryToJob converts the BlockEntry to a BlockEntryJob.
func BlockEntryToJob(entry *BlockEntry) *BlockEntryJob {
	return &BlockEntryJob{Entry: entry}
}
