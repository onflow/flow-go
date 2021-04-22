package verification

import (
	"github.com/onflow/flow-go/model/flow"
)

// ChunkStatus is a data struct represents the current status of fetching chunk data pack for the chunk.
type ChunkStatus struct {
	Chunk             *flow.Chunk
	ExecutionResultID flow.Identifier
}

func (s ChunkStatus) ID() flow.Identifier {
	return s.Chunk.ID()
}

func (s ChunkStatus) Checksum() flow.Identifier {
	return s.Chunk.ID()
}

type ChunkStatusList []*ChunkStatus

func (l ChunkStatusList) Chunks() flow.ChunkList {
	chunks := make(flow.ChunkList, 0, len(l))
	for _, status := range l {
		chunks = append(chunks, status.Chunk)
	}

	return chunks
}
