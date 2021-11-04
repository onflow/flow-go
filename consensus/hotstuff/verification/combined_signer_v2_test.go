package verification

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/module"
	modulemock "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// 1. if the node has beacon keys, then sign with random beacon key
func TestWithBeaconKeys(t *testing.T) {
	// prepare data
	dkgKey := unittest.DKGParticipantPriv()
	pk := dkgKey.RandomBeaconPrivKey.PublicKey()
	signerID := dkgKey.NodeID
	viewComplete := uint64(20)

	fblock := unittest.BlockFixture()
	fblock.Header.ProposerID = signerID
	fblock.Header.View = viewComplete
	block := model.BlockFromFlow(fblock.Header, 10)

	dkg := &mocks.DKG{}
	dkg.On("KeyShare", signerID).Return(pk, nil)

	committee := &mocks.Committee{}
	committee.On("DKG", mock.Anything).Return(dkg, nil)

	thresholdVerifier := &modulemock.ThresholdVerifier{}

	beaconSigner := &modulemock.ThresholdSigner{}
	beaconSigner.On("Sign", mock.Anything).Return(crypto.Signature([]byte{1, 2, 3}), nil).Once()

	thresholdSignerStore := &modulemock.ThresholdSignerStore{}
	// mock the case for DKG complete and has beacon signer
	thresholdSignerStore.On("GetThresholdSigner", viewComplete).Return(beaconSigner, nil)

	staking := &modulemock.AggregatingSigner{}
	signer := NewCombinedSignerV2(committee, staking, thresholdVerifier, thresholdSignerStore, signerID)
	proposal, err := signer.CreateProposal(block)
	require.NoError(t, err)

	sigType, _, err := signature.DecodeSingleSig(proposal.SigData)
	require.NoError(t, err)

	expectedSigType := hotstuff.SigTypeRandomBeacon
	require.Equal(t, expectedSigType, sigType)

	// ensure the random beacon key was used to sign the message
	beaconSigner.AssertExpectations(t)
}

// 2. if DKG was not completed, then sign with staking key
func TestDKGInComplete(t *testing.T) {
	dkgKey := unittest.DKGParticipantPriv()
	pk := dkgKey.RandomBeaconPrivKey.PublicKey()
	signerID := dkgKey.NodeID
	viewIncomplete := uint64(100)

	fblock := unittest.BlockFixture()
	fblock.Header.ProposerID = signerID
	fblock.Header.View = viewIncomplete
	block := model.BlockFromFlow(fblock.Header, 10)

	dkg := &mocks.DKG{}
	dkg.On("KeyShare", signerID).Return(pk, nil)

	committee := &mocks.Committee{}
	committee.On("DKG", mock.Anything).Return(dkg, nil)

	thresholdVerifier := &modulemock.ThresholdVerifier{}

	// mock the case for DKG was incomplete and doesn't have beacon signer
	thresholdSignerStore := &modulemock.ThresholdSignerStore{}
	thresholdSignerStore.On("GetThresholdSigner", viewIncomplete).Return(nil,
		fmt.Errorf("dkg incomplete: %w", module.DKGIncompleteError))

	staking := &modulemock.AggregatingSigner{}
	staking.On("Sign", mock.Anything).Return(crypto.Signature([]byte{1, 2, 3}), nil).Once()
	signer := NewCombinedSignerV2(committee, staking, thresholdVerifier, thresholdSignerStore, signerID)
	proposal, err := signer.CreateProposal(block)
	require.NoError(t, err)

	sigType, _, err := signature.DecodeSingleSig(proposal.SigData)
	require.NoError(t, err)

	expectedSigType := hotstuff.SigTypeStaking
	require.Equal(t, expectedSigType, sigType)

	// ensure the staking key was used to sign the message
	staking.AssertExpectations(t)
}
