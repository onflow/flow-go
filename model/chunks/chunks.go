package chunks

import (
	"github.com/onflow/flow-go/model/flow"
)

// ChunkListFromCommit creates a chunklist with one chunk whose final state is
// the commit
func ChunkListFromCommit(commit flow.StateCommitment) flow.ChunkList {
	chunks := flow.ChunkList{}
	chunk := flow.NewRootChunk(commit)
	chunks.Insert(chunk)

	return chunks
}
