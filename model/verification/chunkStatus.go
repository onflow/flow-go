package verification

import (
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
)

// ChunkStatus is a data struct represents the current status of fetching chunk data pack for the chunk.
type ChunkStatus struct {
	ChunkIndex      uint64
	BlockHeight     uint64
	ExecutionResult *flow.ExecutionResult
}

func (s ChunkStatus) Chunk() *flow.Chunk {
	return s.ExecutionResult.Chunks[s.ChunkIndex]
}

func (s ChunkStatus) ChunkLocatorID() flow.Identifier {
	return chunks.ChunkLocatorID(s.ExecutionResult.ID(), s.ChunkIndex)
}

type ChunkStatusList []*ChunkStatus

func (l ChunkStatusList) Chunks() flow.ChunkList {
	chunks := make(flow.ChunkList, 0, len(l))
	for _, status := range l {
		chunks = append(chunks, status.ExecutionResult.Chunks[status.ChunkIndex])
	}

	return chunks
}
