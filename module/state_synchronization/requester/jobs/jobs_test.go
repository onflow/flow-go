package jobs_test

import (
	"testing"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/state_synchronization/requester/jobs"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/assert"
)

func TestJobID(t *testing.T) {
	fid := unittest.IdentifierFixture()
	jobID := jobs.JobID(fid)

	assert.IsType(t, module.JobID(""), jobID)
	assert.Equal(t, fid.String(), string(jobID))
}

func TestBlockJob(t *testing.T) {
	block := unittest.BlockHeaderFixture()

	job := jobs.BlockToJob(&block)
	assert.IsType(t, &jobs.BlockJob{}, job, "job is not a block job")

	jobID := jobs.JobID(block.ID())
	assert.Equal(t, job.ID(), jobID, "job ID is not the block ID")

	b, err := jobs.JobToBlock(job)
	assert.NoError(t, err, "unexpected error converting block job to block")
	assert.Equal(t, block, *b, "converted block is not the same as the original block")
}

func TestNotifyJob(t *testing.T) {
	blockEntry := &jobs.BlockEntry{
		Height:  42,
		BlockID: unittest.IdentifierFixture(),
	}

	job := jobs.BlockEntryToJob(blockEntry)
	assert.IsType(t, &jobs.NotifyJob{}, job, "job is not a block entry job")

	jobID := jobs.JobID(blockEntry.BlockID)
	assert.Equal(t, job.ID(), jobID, "job ID is not the block ID")

	e, err := jobs.JobToBlockEntry(job)
	assert.NoError(t, err, "unexpected error converting notify job to block entry")
	assert.Equal(t, blockEntry, e, "converted block entry is not the same as the original block entry")
}
