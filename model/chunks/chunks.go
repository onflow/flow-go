package chunks

import (
	"github.com/onflow/flow-go/model/flow"
)

// ChunkListFromCommit creates a chunklist with one chunk whos final state is
// the commit
func ChunkListFromCommit(commit flow.StateCommitment) flow.ChunkList {
	chunks := flow.ChunkList{}
	chunk := flow.NewChunk(
		flow.Identifier{},
		0,
		flow.StateCommitment{},
		0,
		flow.Identifier{},
		0,
		commit,
		0,
	)
	chunks.Insert(chunk)

	return chunks
}
