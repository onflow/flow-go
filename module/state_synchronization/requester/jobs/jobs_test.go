package jobs_test

import (
	"testing"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/jobqueue"
	"github.com/onflow/flow-go/module/state_synchronization/requester/jobs"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/assert"
)

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
		jobID := jobqueue.JobID(blockEntry.BlockID)
		assert.Equal(t, job.ID(), jobID, "job ID is not the block ID")
	})

	t.Run("job converts to block entry", func(t *testing.T) {
		e, err := jobs.JobToBlockEntry(job)
		assert.NoError(t, err, "unexpected error converting notify job to block entry")
		assert.Equal(t, blockEntry, e, "converted block entry is not the same as the original block entry")
	})

	t.Run("incorrect job type fails to convert to block entry", func(t *testing.T) {
		e, err := jobs.JobToBlockEntry(invalidJob{})
		assert.Error(t, err, "expected error converting invalidJob to block entry")
		assert.Nil(t, e, "expected nil block entry")
	})
}

type invalidJob struct{}

func (j invalidJob) ID() module.JobID {
	return "invalid"
}
