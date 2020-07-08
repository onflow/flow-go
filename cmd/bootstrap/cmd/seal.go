package cmd

import (
	"encoding/hex"

	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	model "github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/flow"
)

func constructRootResultAndSeal(rootCommit string, block *flow.Block) {

	commit, err := hex.DecodeString(rootCommit)
	if err != nil {
		log.Fatal().Err(err).Msg("could not decode state commitment")
	}

	result := run.GenerateRootResult(block, commit)
	seal := run.GenerateRootSeal(result)

	writeJSON(model.PathRootResult, result)
	writeJSON(model.PathRootSeal, seal)
}
