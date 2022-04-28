package jobqueue_test

import (
	"testing"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/jobqueue"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/assert"
)

func TestJobID(t *testing.T) {
	fid := unittest.IdentifierFixture()
	jobID := jobqueue.JobID(fid)

	assert.IsType(t, module.JobID(""), jobID)
	assert.Equal(t, fid.String(), string(jobID))
}

func TestBlockJob(t *testing.T) {
	block := unittest.BlockFixture()
	job := jobqueue.BlockToJob(&block)

	t.Run("job is correct type", func(t *testing.T) {
		assert.IsType(t, &jobqueue.BlockJob{}, job, "job is not a block job")
	})

	t.Run("job ID matches block ID", func(t *testing.T) {
		jobID := jobqueue.JobID(block.ID())
		assert.Equal(t, job.ID(), jobID, "job ID is not the block ID")
	})

	t.Run("job converts to block", func(t *testing.T) {
		b, err := jobqueue.JobToBlock(job)
		assert.NoError(t, err, "unexpected error converting notify job to block")
		assert.Equal(t, block, *b, "converted block is not the same as the original block")
	})

	t.Run("incorrect job type fails to convert to block", func(t *testing.T) {
		e, err := jobqueue.JobToBlock(invalidJob{})
		assert.Error(t, err, "expected error converting invalidJob to block")
		assert.Nil(t, e, "expected nil block")
	})
}

func TestBlockHeaderJob(t *testing.T) {
	block := unittest.BlockHeaderFixture()
	job := jobqueue.BlockHeaderToJob(&block)

	t.Run("job is correct type", func(t *testing.T) {
		assert.IsType(t, &jobqueue.BlockHeaderJob{}, job, "job is not a block job")
	})

	t.Run("job ID matches block ID", func(t *testing.T) {
		jobID := jobqueue.JobID(block.ID())
		assert.Equal(t, job.ID(), jobID, "job ID is not the block ID")
	})

	t.Run("job converts to header", func(t *testing.T) {
		b, err := jobqueue.JobToBlockHeader(job)
		assert.NoError(t, err, "unexpected error converting notify job to header")
		assert.Equal(t, block, *b, "converted header is not the same as the original header")
	})

	t.Run("incorrect job type fails to convert to header", func(t *testing.T) {
		e, err := jobqueue.JobToBlockHeader(invalidJob{})
		assert.Error(t, err, "expected error converting invalidJob to header")
		assert.Nil(t, e, "expected nil header")
	})
}

type invalidJob struct{}

func (j invalidJob) ID() module.JobID {
	return "invalid"
}
