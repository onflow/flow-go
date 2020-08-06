package cmd

import (
	"encoding/hex"

	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/committee/leader"
	hotstuff "github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	model "github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/epoch"
	"github.com/dapperlabs/flow-go/model/flow"
)

func constructRootResultAndSeal(
	rootCommit string,
	block *flow.Block,
	participantNodes []model.NodeInfo,
	assignments flow.AssignmentList,
	clusterQCs []*hotstuff.QuorumCertificate,
	dkgData model.DKGData,
) {

	stateCommit, err := hex.DecodeString(rootCommit)
	if err != nil {
		log.Fatal().Err(err).Msg("could not decode state commitment")
	}

	participants := model.ToIdentityList(participantNodes)
	blockID := block.ID()

	epochSetup := &epoch.Setup{
		Counter:      flagEpochCounter,
		FinalView:    block.Header.View + leader.EstimatedSixMonthOfViews,
		Participants: participants,
		Assignments:  assignments,
		Seed:         blockID[:],
	}

	dkgLookup := model.ToDKGLookup(dkgData, participants)

	epochCommit := &epoch.Commit{
		Counter:         flagEpochCounter,
		ClusterQCs:      clusterQCs,
		DKGGroupKey:     dkgData.PubGroupKey,
		DKGParticipants: dkgLookup,
	}

	result := run.GenerateRootResult(block, stateCommit)
	seal := run.GenerateRootSeal(result, epochSetup, epochCommit)

	writeJSON(model.PathRootResult, result)
	writeJSON(model.PathRootSeal, seal)
}
