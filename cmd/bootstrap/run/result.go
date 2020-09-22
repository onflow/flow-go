package run

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

func GenerateRootResult(block *flow.Block, commit flow.StateCommitment) *flow.ExecutionResult {
	chunks := flow.ChunkList{}
	chunk := &flow.Chunk{
		Index:    0,
		EndState: commit,
	}
	chunks.Insert(chunk)

	result := &flow.ExecutionResult{
		ExecutionResultBody: flow.ExecutionResultBody{
			PreviousResultID: flow.ZeroID,
			BlockID:          block.ID(),
			Chunks:           chunks,
		},
		Signatures: nil,
	}
	return result
}
