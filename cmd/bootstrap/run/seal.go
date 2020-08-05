package run

import (
	"github.com/dapperlabs/flow-go/model/epoch"
	"github.com/dapperlabs/flow-go/model/flow"
)

func GenerateRootSeal(result *flow.ExecutionResult, setup *epoch.Setup, commit *epoch.Commit) *flow.Seal {
	seal := &flow.Seal{
		BlockID:       result.BlockID,
		ResultID:      result.ID(),
		InitialState:  nil,
		FinalState:    result.FinalStateCommit,
		ServiceEvents: []interface{}{setup, commit},
	}
	return seal
}
