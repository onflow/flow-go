package verification

import (
	"time"

	"github.com/onflow/flow-go/model/flow"
)

// ChunkRequestStatus is a data struct represents the current status of fetching
// chunk data pack for the chunk.
type ChunkRequestStatus struct {
	*ChunkDataPackRequest
	LastAttempt time.Time
	Attempt     int
}

func (c ChunkRequestStatus) ID() flow.Identifier {
	return c.ChunkID
}

func (c ChunkRequestStatus) Checksum() flow.Identifier {
	return c.ChunkID
}
