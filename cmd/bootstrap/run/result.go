package run

import (
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
)

func GenerateRootResult(block *flow.Block, commit flow.StateCommitment) *flow.ExecutionResult {
	result := &flow.ExecutionResult{
		PreviousResultID: flow.ZeroID,
		BlockID:          block.ID(),
		Chunks:           chunks.ChunkListFromCommit(commit),
	}
	return result
}
