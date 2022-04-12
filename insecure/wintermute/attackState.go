package wintermute

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// attackState keeps data structures related to a specific wintermute attack instance.
type attackState struct {
	originalResult  *flow.ExecutionResult // original valid execution result
	corruptedResult *flow.ExecutionResult // corrupted execution result by orchestrator
}

// containsCorruptedChunkId returns true if corrupted result of this attack state contains a chunk with the given chunk id, and otherwise, false.
func (a attackState) containsCorruptedChunkId(chunkId flow.Identifier) bool {
	return flow.IdentifierList(flow.GetIDs(a.corruptedResult.Chunks)).Contains(chunkId)
}

// corruptedChunkIndexOf returns the chunk index of the corresponding corrupted chunk id.
func (a attackState) corruptedChunkIndexOf(chunkId flow.Identifier) (uint64, error) {
	for _, chunk := range a.corruptedResult.Chunks {
		if chunk.ID() == chunkId {
			return chunk.Index, nil
		}
	}

	return 0, fmt.Errorf("could not lookup chunk id in corrupted result: %x", chunkId)
}
