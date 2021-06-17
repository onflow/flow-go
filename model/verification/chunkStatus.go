package verification

import (
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
)

// ChunkStatus is a data struct represents the current status of fetching chunk data pack for the chunk.
type ChunkStatus struct {
	ChunkIndex      uint64
	ExecutionResult *flow.ExecutionResult
	BlockHeight     uint64
}

func (s ChunkStatus) ID() flow.Identifier {
	return s.ExecutionResult.Chunks[s.ChunkIndex].ID()
}

func (s ChunkStatus) Checksum() flow.Identifier {
	return s.ExecutionResult.Chunks[s.ChunkIndex].ID()
}

func (s ChunkStatus) ChunkLocatorID() flow.Identifier {
	return chunks.Locator{
		ResultID: s.ExecutionResult.ID(),
		Index:    s.ChunkIndex,
	}.ID()
}

type ChunkStatusList []*ChunkStatus

func (l ChunkStatusList) Chunks() flow.ChunkList {
	chunks := make(flow.ChunkList, 0, len(l))
	for _, status := range l {
		chunks = append(chunks, status.ExecutionResult.Chunks[status.ChunkIndex])
	}

	return chunks
}
