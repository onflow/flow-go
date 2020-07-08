package bootstrap

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

func Result(block *flow.Block, commit flow.StateCommitment) *flow.ExecutionResult {
	result := &flow.ExecutionResult{
		ExecutionResultBody: flow.ExecutionResultBody{
			BlockID:          block.ID(),
			PreviousResultID: flow.ZeroID,
			FinalStateCommit: commit,
			Chunks:           nil,
		},
		Signatures: nil,
	}
	return result
}
