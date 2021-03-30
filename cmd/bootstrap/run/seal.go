package run

import (
	"github.com/onflow/flow-go/model/flow"
)

func GenerateRootSeal(result *flow.ExecutionResult, setup *flow.EpochSetup, commit *flow.EpochCommit) *flow.Seal {
	finalState := result.FinalStateCommitment()
	seal := &flow.Seal{
		BlockID:       result.BlockID,
		ResultID:      result.ID(),
		FinalState:    finalState,
		ServiceEvents: []flow.ServiceEvent{setup.ServiceEvent(), commit.ServiceEvent()},
	}
	return seal
}
