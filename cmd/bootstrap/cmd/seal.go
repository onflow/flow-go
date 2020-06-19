package cmd

import (
	"encoding/hex"

	model "github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/flow"
)

func constructRootResultAndSeal(commitString string, block *flow.Block) {

	commit, err := hex.DecodeString(commitString)
	if err != nil {
		log.Fatal().Err(err).Msg("could not decode state commitment")
	}

	blockID := block.ID()

	result := &flow.ExecutionResult{
		ExecutionResultBody: flow.ExecutionResultBody{
			PreviousResultID: flow.ZeroID,
			BlockID:          blockID,
			FinalStateCommit: commit,
			Chunks:           nil,
		},
		Signatures: nil,
	}

	seal := &flow.Seal{
		BlockID:      block.ID(),
		ResultID:     result.ID(),
		InitialState: nil,
		FinalState:   commit,
	}

	writeJSON(model.PathRootResult, result)
	writeJSON(model.PathRootSeal, seal)
}
