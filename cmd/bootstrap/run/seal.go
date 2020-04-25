package run

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

func GenerateRootSeal(commit flow.StateCommitment) flow.Seal {
	return flow.Seal{
		BlockID:      flow.ZeroID, // empty because it seals a state before genesis block
		ResultID:     flow.ZeroID, // empty because it seals a state before any execution result
		InitialState: nil,         // empty because there is no related execution result
		FinalState:   commit,      // this is the initial hash of the execution state
	}
}
