package cmd

import (
	"crypto/rand"
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

	stateCommitBytes, err := hex.DecodeString(rootCommit)
	if err != nil {
		log.Fatal().Err(err).Msg("could not decode state commitment")
	}
	stateCommit, err := flow.ToStateCommitment(stateCommitBytes)
	if err != nil {
		log.Fatal().
			Int("expected_state_commitment_length", len(stateCommit)).
			Int("received_state_commitment_length", len(stateCommitBytes)).
			Msg("root state commitment has incompatible length")
	}

	participants := model.ToIdentityList(participantNodes)

	randomSource := make([]byte, flow.EpochSetupRandomSourceLength)
	_, err = rand.Read(randomSource)
	if err != nil {
		log.Fatal().Err(err).Msg("could not generate random source for epoch setup event")
	}

	epochSetup := &flow.EpochSetup{
		Counter:      flagEpochCounter,
		FirstView:    block.Header.View,
		FinalView:    block.Header.View + leader.EstimatedSixMonthOfViews,
		Participants: participants,
		Assignments:  assignments,
		RandomSource: randomSource,
	}

	dkgLookup := model.ToDKGLookup(dkgData, participants)

	epochCommit := &flow.EpochCommit{
		Counter:         flagEpochCounter,
		ClusterQCs:      clusterQCs,
		DKGGroupKey:     dkgData.PubGroupKey,
		DKGParticipants: dkgLookup,
	}

	result := run.GenerateRootResult(block, stateCommit, epochSetup, epochCommit)
	seal, err := run.GenerateRootSeal(result)
	if err != nil {
		log.Fatal().Err(err).Msg("could not generate root seal")
	}

	return result, seal
}
