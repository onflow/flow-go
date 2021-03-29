package cmd

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/onflow/flow-go/cmd/bootstrap/run"
	"github.com/onflow/flow-go/consensus/hotstuff/committees/leader"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/dkg"
	"github.com/onflow/flow-go/model/flow"
)

func constructRootResultAndSeal(
	rootCommit string,
	block *flow.Block,
	participantNodes []bootstrap.NodeInfo,
	assignments flow.AssignmentList,
	clusterQCs []*flow.QuorumCertificate,
	dkgData dkg.DKGData,
) (*flow.ExecutionResult, *flow.Seal) {

	stateCommit, err := hex.DecodeString(rootCommit)
	if err != nil {
		log.Fatal().Err(err).Msg("could not decode state commitment")
	}

	participants := bootstrap.ToIdentityList(participantNodes)

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

	dkgLookup := dkg.ToDKGLookup(dkgData, participants)

	epochCommit := &flow.EpochCommit{
		Counter:         flagEpochCounter,
		ClusterQCs:      clusterQCs,
		DKGGroupKey:     dkgData.PubGroupKey,
		DKGParticipants: dkgLookup,
	}

	result := run.GenerateRootResult(block, stateCommit, epochSetup, epochCommit)
	seal := run.GenerateRootSeal(result)

	return result, seal
}
