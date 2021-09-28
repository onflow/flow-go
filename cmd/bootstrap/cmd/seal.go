package cmd

import (
	"encoding/hex"

	"github.com/onflow/flow-go/cmd/bootstrap/run"
	"github.com/onflow/flow-go/model/dkg"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/order"
)

func constructRootResultAndSeal(
	rootCommit string,
	block *flow.Block,
	participants flow.IdentityList,
	assignments flow.AssignmentList,
	clusterQCs []*flow.QuorumCertificate,
	dkgData dkg.DKGData,
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

	firstView := block.Header.View
	epochSetup := &flow.EpochSetup{
		Counter:            flagEpochCounter,
		FirstView:          firstView,
		FinalView:          firstView + flagNumViewsInEpoch - 1,
		DKGPhase1FinalView: firstView + flagNumViewsInStakingAuction + flagNumViewsInDKGPhase - 1,
		DKGPhase2FinalView: firstView + flagNumViewsInStakingAuction + flagNumViewsInDKGPhase*2 - 1,
		DKGPhase3FinalView: firstView + flagNumViewsInStakingAuction + flagNumViewsInDKGPhase*3 - 1,
		Participants:       participants.Sort(order.Canonical),
		Assignments:        assignments,
		RandomSource:       getRandomSource(flagBootstrapRandomSeed),
	}

	epochCommit := &flow.EpochCommit{
		Counter:            flagEpochCounter,
		ClusterQCs:         flow.ClusterQCVoteDatasFromQCs(clusterQCs),
		DKGGroupKey:        dkgData.PubGroupKey,
		DKGParticipantKeys: dkgData.PubKeyShares,
	}

	result := run.GenerateRootResult(block, stateCommit, epochSetup, epochCommit)
	seal, err := run.GenerateRootSeal(result)
	if err != nil {
		log.Fatal().Err(err).Msg("could not generate root seal")
	}

	if seal.ResultID != result.ID() {
		log.Fatal().Msgf("root block seal (%v) mismatch with result id: (%v)", seal.ResultID, result.ID())
	}

	return result, seal
}

// getRandomSource produces the random source which is included in the root
// EpochSetup event and is used for leader selection. The random source is
// generated deterministically based on the seed, so that the
// bootstrapping files are deterministic across runs for the same inputs.
func getRandomSource(seed []byte) []byte {

	if len(seed) < flow.EpochSetupRandomSourceLength {
		log.Fatal().Msgf("seed length is smaller than required random source length (%d < %d)", len(seed), flow.EpochSetupRandomSourceLength)
	}

	randomSource := make([]byte, flow.EpochSetupRandomSourceLength)
	copy(randomSource, seed)

	return randomSource
}
