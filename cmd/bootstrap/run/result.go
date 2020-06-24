package run

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

func GenerateRootResult(block *flow.Block, commit flow.StateCommitment) *flow.ExecutionResult {
	result := &flow.ExecutionResult{
		ExecutionResultBody: flow.ExecutionResultBody{
			PreviousResultID: flow.ZeroID,
			BlockID:          block.ID(),
			FinalStateCommit: commit,
			Chunks:           nil,
		},
		Signatures: nil,
	}
	return result
}
