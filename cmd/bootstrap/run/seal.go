package run

import (
	"github.com/onflow/flow-go/model/flow"
)

func GenerateRootSeal(result *flow.ExecutionResult) *flow.Seal {
	finalState, _ := result.FinalStateCommitment()
	seal := &flow.Seal{
		BlockID:    result.BlockID,
		ResultID:   result.ID(),
		FinalState: finalState,
	}
	return seal
}
