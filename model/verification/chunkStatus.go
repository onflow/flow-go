package verification

import (
	"github.com/onflow/flow-go/model/flow"
)

// ChunkStatus is a data struct represents the current status of fetching chunk data pack for the chunk.
type ChunkStatus struct {
	ChunkIndex      uint64
	ExecutionResult *flow.ExecutionResult
	ChunkLocatorID  flow.Identifier // used to notify back chunk consumer once processing chunk is done.
}

func (s ChunkStatus) ID() flow.Identifier {
	return s.ExecutionResult.Chunks[s.ChunkIndex].ID()
}

func (s ChunkStatus) Checksum() flow.Identifier {
	return s.ExecutionResult.Chunks[s.ChunkIndex].ID()
}

type ChunkStatusList []*ChunkStatus

func (l ChunkStatusList) Chunks() flow.ChunkList {
	chunks := make(flow.ChunkList, 0, len(l))
	for _, status := range l {
		chunks = append(chunks, status.ExecutionResult.Chunks[status.ChunkIndex])
	}

	return chunks
}
