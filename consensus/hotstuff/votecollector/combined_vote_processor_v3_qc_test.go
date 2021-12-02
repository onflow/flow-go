// use a different package to avoid import cycle, because this package imports
// github.com/onflow/flow-go/cmd/bootstrap/run,  which imports
// github.com/onflow/flow-go/consensus/hotstuff/votecollector
package votecollector_test

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/bootstrap/run"
	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	mockhotstuff "github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/signature"
	hsig "github.com/onflow/flow-go/consensus/hotstuff/signature"
	hotstuffvalidator "github.com/onflow/flow-go/consensus/hotstuff/validator"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	"github.com/onflow/flow-go/consensus/hotstuff/votecollector"
	"github.com/onflow/flow-go/model/dkg"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/local"
	modulemock "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/state/protocol/inmem"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestCombinedVoteProcessorV3_BuildVerifyQC tests a complete path from creating votes to collecting votes and then
// building & verifying QC.
// We start with leader proposing a block, then new leader collects votes and builds a QC.
// Need to verify that QC that was produced is valid and can be embedded in new proposal.
func TestCombinedVoteProcessorV3_BuildVerifyQC(t *testing.T) {
	epochCounter := uint64(3)
	epochLookup := &modulemock.EpochLookup{}
	view := uint64(20)
	epochLookup.On("EpochForViewWithFallback", view).Return(epochCounter, nil)

	dkgData, err := run.RunFastKG(11, unittest.RandomBytes(32))
	require.NoError(t, err)

	// signers hold objects that are created with private key and can sign votes and proposals
	signers := make(map[flow.Identifier]*verification.CombinedSignerV3)

	// prepare staking signers, each signer has it's own private/public key pair
	// stakingSigners sign only with staking key, meaning they have failed DKG
	stakingSigners := unittest.IdentityListFixture(3)
	beaconSigners := unittest.IdentityListFixture(8)
	allIdentities := append(stakingSigners, beaconSigners...)
	require.Equal(t, len(dkgData.PubKeyShares), len(allIdentities))
	dkgParticipants := make(map[flow.Identifier]flow.DKGParticipant)
	// fill dkg participants data
	for index, identity := range allIdentities {
		dkgParticipants[identity.NodeID] = flow.DKGParticipant{
			Index:    uint(index),
			KeyShare: dkgData.PubKeyShares[index],
		}
	}

	for _, identity := range stakingSigners {
		stakingPriv := unittest.StakingPrivKeyFixture()
		identity.StakingPubKey = stakingPriv.PublicKey()

		keys := &storagemock.DKGKeys{}
		// there is no DKG key for this epoch
		keys.On("RetrieveMyDKGPrivateInfo", epochCounter).Return(nil, false, nil)

		beaconSignerStore := hsig.NewEpochAwareRandomBeaconKeyStore(epochLookup, keys)

		me, err := local.New(identity, stakingPriv)
		require.NoError(t, err)

		signers[identity.NodeID] = verification.NewCombinedSignerV3(me, beaconSignerStore)
	}

	for _, identity := range beaconSigners {
		stakingPriv := unittest.StakingPrivKeyFixture()
		identity.StakingPubKey = stakingPriv.PublicKey()

		participantData := dkgParticipants[identity.NodeID]

		dkgKey := &dkg.DKGParticipantPriv{
			NodeID: identity.NodeID,
			RandomBeaconPrivKey: encodable.RandomBeaconPrivKey{
				PrivateKey: dkgData.PrivKeyShares[participantData.Index],
			},
			GroupIndex: int(participantData.Index),
		}

		keys := &storagemock.DKGKeys{}
		// there is DKG key for this epoch
		keys.On("RetrieveMyDKGPrivateInfo", epochCounter).Return(dkgKey, true, nil)

		beaconSignerStore := hsig.NewEpochAwareRandomBeaconKeyStore(epochLookup, keys)

		me, err := local.New(identity, stakingPriv)
		require.NoError(t, err)

		signers[identity.NodeID] = verification.NewCombinedSignerV3(me, beaconSignerStore)
	}

	leader := stakingSigners[0]

	block := helper.MakeBlock(helper.WithBlockView(view),
		helper.WithBlockProposer(leader.NodeID))

	inmemDKG, err := inmem.DKGFromEncodable(inmem.EncodableDKG{
		GroupKey: encodable.RandomBeaconPubKey{
			PublicKey: dkgData.PubGroupKey,
		},
		Participants: dkgParticipants,
	})
	require.NoError(t, err)

	committee := &mockhotstuff.Committee{}
	committee.On("Identities", block.BlockID, mock.Anything).Return(allIdentities, nil)
	committee.On("DKG", block.BlockID).Return(inmemDKG, nil)

	votes := make([]*model.Vote, 0, len(allIdentities))

	// first staking signer will be leader collecting votes for proposal
	// prepare votes for every member of committee except leader
	for _, signer := range allIdentities[1:] {
		vote, err := signers[signer.NodeID].CreateVote(block)
		require.NoError(t, err)
		votes = append(votes, vote)
	}

	// create and sign proposal
	proposal, err := signers[leader.NodeID].CreateProposal(block)
	require.NoError(t, err)

	qcCreated := false
	onQCCreated := func(qc *flow.QuorumCertificate) {
		packer := signature.NewConsensusSigDataPacker(committee)

		// create verifier that will do crypto checks of created QC
		verifier := verification.NewCombinedVerifierV3(committee, packer)
		forks := &mockhotstuff.Forks{}
		// create validator which will do compliance and crypto checked of created QC
		validator := hotstuffvalidator.New(committee, forks, verifier)
		// check if QC is valid against parent
		err := validator.ValidateQC(qc, block)
		require.NoError(t, err)

		qcCreated = true
	}

	voteProcessorFactory := votecollector.NewCombinedVoteProcessorFactory(unittest.Logger(), committee, onQCCreated)
	voteProcessor, err := voteProcessorFactory.Create(proposal)
	require.NoError(t, err)

	// process votes by new leader, this will result in producing new QC
	for _, vote := range votes {
		err := voteProcessor.Process(vote)
		require.NoError(t, err)
	}

	require.True(t, qcCreated)
}
