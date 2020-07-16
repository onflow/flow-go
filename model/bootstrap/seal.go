package bootstrap

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

func Seal(result *flow.ExecutionResult) *flow.Seal {
	seal := &flow.Seal{
		BlockID:      result.BlockID,
		ResultID:     result.ID(),
		InitialState: nil,
		FinalState:   result.FinalStateCommit,
	}
	return seal
}
