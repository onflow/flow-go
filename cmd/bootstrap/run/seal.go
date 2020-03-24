package run

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

func GenerateRootSeal(sc flow.StateCommitment) flow.Seal {
	return flow.Seal{
		BlockID:           flow.ZeroID, // empty because the seal is just for a previous state, not a block
		ExecutionResultID: flow.ZeroID, // empty because the seal is just for a previous state, not an ER
		FinalState:        sc,
		PreviousState:     nil, // empty because there is no previous seal
		Signature:         nil, // empty because the first seal is trusted, not signed
	}
}
