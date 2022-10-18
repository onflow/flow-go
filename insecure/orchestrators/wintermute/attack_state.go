package wintermute

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// attackState keeps data structures related to a specific wintermute attack instance.
type attackState struct {
	originalResult *flow.ExecutionResult // original valid execution result
	corruptResult  *flow.ExecutionResult // corrupt execution result by orchestrator
}

// containsCorruptChunkId returns true if corrupt result of this attack state contains a chunk with the given chunk id, and otherwise, false.
func (a attackState) containsCorruptChunkId(chunkId flow.Identifier) bool {
	return flow.IdentifierList(flow.GetIDs(a.corruptResult.Chunks)).Contains(chunkId)
}

// corruptChunkIndexOf returns the chunk index of the corresponding corrupt chunk id.
func (a attackState) corruptChunkIndexOf(chunkId flow.Identifier) (uint64, error) {
	for _, chunk := range a.corruptResult.Chunks {
		if chunk.ID() == chunkId {
			return chunk.Index, nil
		}
	}

	return 0, fmt.Errorf("could not lookup chunk id in corrupt result: %x", chunkId)
}
