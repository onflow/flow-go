package cmd

import (
	"encoding/hex"

	"github.com/onflow/flow-go/cmd/bootstrap/run"
	"github.com/onflow/flow-go/model/dkg"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/order"
	"github.com/onflow/flow-go/module/signature"
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
		RandomSource:       GenerateRandomSeed(flow.EpochSetupRandomSourceLength),
		TargetEndTime:      0, // TODO
	}

	qcsWithSignerIDs := make([]*flow.QuorumCertificateWithSignerIDs, 0, len(clusterQCs))
	for i, clusterQC := range clusterQCs {
		members := assignments[i]
		signerIDs, err := signature.DecodeSignerIndicesToIdentifiers(members, clusterQC.SignerIndices)
		if err != nil {
			log.Fatal().Err(err).Msgf("could not decode signer IDs from clusterQC at index %v", i)
		}
		qcsWithSignerIDs = append(qcsWithSignerIDs, &flow.QuorumCertificateWithSignerIDs{
			View:      clusterQC.View,
			BlockID:   clusterQC.BlockID,
			SignerIDs: signerIDs,
			SigData:   clusterQC.SigData,
		})
	}

	epochCommit := &flow.EpochCommit{
		Counter:            flagEpochCounter,
		ClusterQCs:         flow.ClusterQCVoteDatasFromQCs(qcsWithSignerIDs),
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

// epochTargetEndTime computes the target end time for the given epoch, using the given config.
func epochTargetEndTime(targetCounter, refCounter, refTimestamp, duration uint64) uint64 {
	return refTimestamp + (targetCounter-refCounter)*duration
}
