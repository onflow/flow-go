package cmd

import (
	"encoding/hex"

	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	model "github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/epoch"
	"github.com/dapperlabs/flow-go/model/flow"
)

func constructRootResultAndSeal(rootCommit string, block *flow.Block, participantNodes []model.NodeInfo, assignments flow.AssignmentList) {

	stateCommit, err := hex.DecodeString(rootCommit)
	if err != nil {
		log.Fatal().Err(err).Msg("could not decode state commitment")
	}

	participants := model.ToIdentityList(participantNodes)

	blockID := block.ID()
	epochSetup := epoch.Setup{
		Counter:      1,                        // TODO flag
		FinalView:    block.Header.View + 1000, // TODO constant
		Participants: participants,
		Assignments:  assignments,
		Seed:         blockID[:],
	}

	epochCommit := epoch.Commit{
		Counter:         1,   // TODO flag
		ClusterQCs:      nil, // TODO
		DKGGroupKey:     nil, // TODO
		DKGParticipants: nil, // TODO
	}

	result := run.GenerateRootResult(block, stateCommit)
	seal := run.GenerateRootSeal(result)

	writeJSON(model.PathRootResult, result)
	writeJSON(model.PathRootSeal, seal)
}
