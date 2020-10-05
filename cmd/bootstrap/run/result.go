package run

import (
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
)

func GenerateRootResult(block *flow.Block, commit flow.StateCommitment) *flow.ExecutionResult {
	result := &flow.ExecutionResult{
		ExecutionResultBody: flow.ExecutionResultBody{
			PreviousResultID: flow.ZeroID,
			BlockID:          block.ID(),
			Chunks:           chunks.ChunkListFromCommit(commit),
		},
		Signatures: nil,
	}
	return result
}
