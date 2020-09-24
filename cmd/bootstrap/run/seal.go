package run

import (
	"github.com/onflow/flow-go/model/flow"
)

func GenerateRootSeal(result *flow.ExecutionResult, setup *flow.EpochSetup, commit *flow.EpochCommit) *flow.Seal {
	seal := &flow.Seal{
		BlockID:       result.BlockID,
		ResultID:      result.ID(),
		InitialState:  nil,
		FinalState:    result.FinalStateCommit,
		ServiceEvents: []flow.ServiceEvent{setup.ServiceEvent(), commit.ServiceEvent()},
	}
	return seal
}
