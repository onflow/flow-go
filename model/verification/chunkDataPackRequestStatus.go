package verification

import (
	"time"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
)

// ChunkRequestStatus is a data struct represents the current status of fetching
// chunk data pack for the chunk.
type ChunkRequestStatus struct {
	*ChunkDataPackRequest
	Targets     flow.IdentityList
	LastAttempt time.Time
	RetryAfter  time.Duration
	Attempt     int
}

func (c ChunkRequestStatus) ID() flow.Identifier {
	return c.ChunkID
}

func (c ChunkRequestStatus) Checksum() flow.Identifier {
	return c.ChunkID
}

// SampleTargets returns identifier of execution nodes that can be asked for the chunk data pack, based on
// the agree and disagree execution nodes of the chunk data pack request.
func (c ChunkRequestStatus) SampleTargets(count int) flow.IdentifierList {
	// if there are enough receipts produced the same result (agrees), we sample from them.
	if len(c.Agrees) >= count {
		return c.Targets.Filter(filter.HasNodeID(c.Agrees...)).Sample(uint(count)).NodeIDs()
	}

	// since there is at least one agree, then usually, we just need `count - 1` extra nodes as backup.
	// We pick these extra nodes randomly from the rest nodes who we haven't received its receipt.
	// In the case where all other execution nodes has produced different results, then we will only
	// fetch from the one produced the same result (the only agree)
	need := uint(count - len(c.Agrees))

	nonResponders := c.Targets.Filter(filter.Not(filter.HasNodeID(c.Disagrees...))).Sample(need).NodeIDs()
	return append(c.Agrees, nonResponders...)
}
