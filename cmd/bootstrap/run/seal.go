package run

import "github.com/dapperlabs/flow-go/model/flow"

func GenerateRootSeal(result *flow.ExecutionResult) *flow.Seal {
	seal := &flow.Seal{
		BlockID:      result.BlockID,
		ResultID:     result.ID(),
		InitialState: nil,
		FinalState:   result.FinalStateCommit,
	}
	return seal
}
