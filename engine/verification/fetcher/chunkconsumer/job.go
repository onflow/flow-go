package chunkconsumer

import (
	"fmt"

	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// ChunkJob converts a chunk locator into a Job to be used by job queue.
type ChunkJob struct {
	ChunkLocator *chunks.Locator
}

// ID converts chunk locator identifier into job id, which guarantees uniqueness.
func (j ChunkJob) ID() module.JobID {
	return locatorIDToJobID(j.ChunkLocator.ID())
}

func locatorIDToJobID(locatorID flow.Identifier) module.JobID {
	return module.JobID(fmt.Sprintf("%v", locatorID))
}

func ChunkLocatorToJob(locator *chunks.Locator) *ChunkJob {
	return &ChunkJob{ChunkLocator: locator}
}

func JobToChunkLocator(job module.Job) (*chunks.Locator, error) {
	chunkjob, ok := job.(*ChunkJob)
	if !ok {
		return nil, fmt.Errorf("could not assert job to chunk locator, job id: %x", job.ID())
	}
	return chunkjob.ChunkLocator, nil
}
