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
	t.Run("job is correct type", func(t *testing.T) {
		assert.IsType(t, &jobs.BlockJob{}, job, "job is not a block job")
	})

	t.Run("job ID matches block ID", func(t *testing.T) {
		jobID := jobs.JobID(block.ID())
		assert.Equal(t, job.ID(), jobID, "job ID is not the block ID")
	})

	t.Run("job converts to block", func(t *testing.T) {
		b, err := jobs.JobToBlock(job)
		assert.NoError(t, err, "unexpected error converting notify job to block")
		assert.Equal(t, block, *b, "converted block is not the same as the original block")
	})

	t.Run("incorrect job type fails to convert to block", func(t *testing.T) {
		blockEntryJob := jobs.BlockEntryToJob(&jobs.BlockEntry{})

		e, err := jobs.JobToBlock(blockEntryJob)
		assert.Error(t, err, "expected error converting block job to block")
		assert.Nil(t, e, "expected nil block")
	})
}

func TestBlockEntryJob(t *testing.T) {
	blockEntry := &jobs.BlockEntry{
		Height:  42,
		BlockID: unittest.IdentifierFixture(),
	}

	job := jobs.BlockEntryToJob(blockEntry)
	t.Run("job is correct type", func(t *testing.T) {
		assert.IsType(t, &jobs.BlockEntryJob{}, job, "job is not a block entry job")
	})

	t.Run("job ID matches block ID", func(t *testing.T) {
		jobID := jobs.JobID(blockEntry.BlockID)
		assert.Equal(t, job.ID(), jobID, "job ID is not the block ID")
	})

	t.Run("job converts to block entry", func(t *testing.T) {
		e, err := jobs.JobToBlockEntry(job)
		assert.NoError(t, err, "unexpected error converting notify job to block entry")
		assert.Equal(t, blockEntry, e, "converted block entry is not the same as the original block entry")
	})

	t.Run("incorrect job type fails to convert to block entry", func(t *testing.T) {
		block := unittest.BlockHeaderFixture()
		blockJob := jobs.BlockToJob(&block)

		e, err := jobs.JobToBlockEntry(blockJob)
		assert.Error(t, err, "expected error converting block job to block entry")
		assert.Nil(t, e, "expected nil block entry")
	})
}
