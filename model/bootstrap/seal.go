package bootstrap

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// TODO need to make this include service events
func Seal(result *flow.ExecutionResult) *flow.Seal {
	seal := &flow.Seal{
		BlockID:      result.BlockID,
		ResultID:     result.ID(),
		InitialState: nil,
		FinalState:   result.FinalStateCommit,
	}
	return seal
}
