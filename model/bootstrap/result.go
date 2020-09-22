package bootstrap

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

func Result(block *flow.Block, commit flow.StateCommitment) *flow.ExecutionResult {
	chunks := flow.ChunkList{}
	chunk := &flow.Chunk{
		Index:    0,
		EndState: commit,
	}

	chunks.Insert(chunk)

	result := &flow.ExecutionResult{
		ExecutionResultBody: flow.ExecutionResultBody{
			BlockID:          block.ID(),
			PreviousResultID: flow.ZeroID,
			Chunks:           chunks,
		},
		Signatures: nil,
	}
	return result
}
