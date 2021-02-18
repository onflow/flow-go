package cmd

import (
	"encoding/hex"

	"github.com/onflow/flow-go/cmd/bootstrap/run"
	"github.com/onflow/flow-go/consensus/hotstuff/committees/leader"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
)

func constructRootResultAndSeal(
	rootCommit string,
	block *flow.Block,
	participantNodes []model.NodeInfo,
	assignments flow.AssignmentList,
	clusterQCs []*flow.QuorumCertificate,
	dkgData model.DKGData,
) (*flow.ExecutionResult, *flow.Seal) {

	stateCommit, err := hex.DecodeString(rootCommit)
	if err != nil {
		log.Fatal().Err(err).Msg("could not decode state commitment")
	}

	participants := model.ToIdentityList(participantNodes)
	blockID := block.ID()

	epochSetup := &flow.EpochSetup{
		Counter:      flagEpochCounter,
		FirstView:    block.Header.View,
		FinalView:    block.Header.View + leader.EstimatedSixMonthOfViews,
		Participants: participants,
		Assignments:  assignments,
		RandomSource: blockID[:],
	}

	dkgLookup := model.ToDKGLookup(dkgData, participants)

	epochCommit := &flow.EpochCommit{
		Counter:         flagEpochCounter,
		ClusterQCs:      clusterQCs,
		DKGGroupKey:     dkgData.PubGroupKey,
		DKGParticipants: dkgLookup,
	}

	result := run.GenerateRootResult(block, stateCommit)
	seal := run.GenerateRootSeal(result, epochSetup, epochCommit)

	// writeJSON(model.PathRootResult, result)
	// writeJSON(model.PathRootSeal, seal)

	return result, seal
}
