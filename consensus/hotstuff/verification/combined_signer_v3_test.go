package verification

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/module/local"
	modulemock "github.com/onflow/flow-go/module/mock"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// Test that when DKG key is available for a view, a signed block can pass the validation
// the sig is a random beacon sig.
func TestCombinedSignWithDKGKeyV3(t *testing.T) {
	// prepare data
	dkgKey := unittest.RandomBeaconPriv()
	pk := dkgKey.PublicKey()
	view := uint64(20)

	fblock := unittest.BlockFixture()
	fblock.Header.View = view
	block := model.BlockFromFlow(fblock.Header, 10)
	signerID := fblock.Header.ProposerID

	epochCounter := uint64(3)
	epochLookup := &modulemock.EpochLookup{}
	epochLookup.On("EpochForViewWithFallback", view).Return(epochCounter, nil)

	keys := &storagemock.SafeBeaconKeys{}
	// there is DKG key for this epoch
	keys.On("RetrieveMyBeaconPrivateKey", epochCounter).Return(dkgKey, true, nil)

	beaconKeyStore := signature.NewEpochAwareRandomBeaconKeyStore(epochLookup, keys)

	stakingPriv := unittest.StakingPrivKeyFixture()
	nodeID := unittest.IdentityFixture()
	nodeID.NodeID = signerID
	nodeID.StakingPubKey = stakingPriv.PublicKey()

	me, err := local.New(nodeID, stakingPriv)
	require.NoError(t, err)
	signer := NewCombinedSignerV3(me, beaconKeyStore)

	dkg := &mocks.DKG{}
	dkg.On("KeyShare", signerID).Return(pk, nil)

	committee := &mocks.Committee{}
	committee.On("DKG", mock.Anything).Return(dkg, nil)

	packer := signature.NewConsensusSigDataPacker(committee)
	verifier := NewCombinedVerifierV3(committee, packer)

	// check that a created proposal can be verified by a verifier
	proposal, err := signer.CreateProposal(block)
	require.NoError(t, err)

	vote := proposal.ProposerVote()
	err = verifier.VerifyVote(nodeID, vote.SigData, proposal.Block)
	require.NoError(t, err)

	// check that a created proposal's signature is a combined staking sig and random beacon sig
	msg := MakeVoteMessage(block.View, block.BlockID)

	beaconSig, err := dkgKey.Sign(msg, crypto.NewBLSKMAC(encoding.RandomBeaconTag))
	require.NoError(t, err)

	expectedSig := signature.EncodeSingleSig(hotstuff.SigTypeRandomBeacon, beaconSig)
	require.Equal(t, expectedSig, proposal.SigData)
}

// Test that when DKG key is not available for a view, a signed block can pass the validation
// the sig is a staking sig
func TestCombinedSignWithNoDKGKeyV3(t *testing.T) {
	// prepare data
	dkgKey := unittest.RandomBeaconPriv()
	pk := dkgKey.PublicKey()
	view := uint64(20)

	fblock := unittest.BlockFixture()
	fblock.Header.View = view
	block := model.BlockFromFlow(fblock.Header, 10)
	signerID := fblock.Header.ProposerID

	epochCounter := uint64(3)
	epochLookup := &modulemock.EpochLookup{}
	epochLookup.On("EpochForViewWithFallback", view).Return(epochCounter, nil)

	keys := &storagemock.SafeBeaconKeys{}
	// there is no DKG key for this epoch
	keys.On("RetrieveMyBeaconPrivateKey", epochCounter).Return(nil, false, nil)

	beaconKeyStore := signature.NewEpochAwareRandomBeaconKeyStore(epochLookup, keys)

	stakingPriv := unittest.StakingPrivKeyFixture()
	nodeID := unittest.IdentityFixture()
	nodeID.NodeID = signerID
	nodeID.StakingPubKey = stakingPriv.PublicKey()

	me, err := local.New(nodeID, stakingPriv)
	require.NoError(t, err)
	signer := NewCombinedSignerV3(me, beaconKeyStore)

	dkg := &mocks.DKG{}
	dkg.On("KeyShare", signerID).Return(pk, nil)

	committee := &mocks.Committee{}
	// even if the node failed DKG, and has no random beacon private key,
	// but other nodes, who completed and succeeded DKG, have a public key
	// for this failed node, which can be used to verify signature from
	// this failed node.
	committee.On("DKG", mock.Anything).Return(dkg, nil)

	packer := signature.NewConsensusSigDataPacker(committee)
	verifier := NewCombinedVerifierV3(committee, packer)

	proposal, err := signer.CreateProposal(block)
	require.NoError(t, err)

	vote := proposal.ProposerVote()
	err = verifier.VerifyVote(nodeID, vote.SigData, proposal.Block)
	require.NoError(t, err)

	// check that a created proposal's signature is a combined staking sig and random beacon sig
	msg := MakeVoteMessage(block.View, block.BlockID)
	stakingSig, err := stakingPriv.Sign(msg, crypto.NewBLSKMAC(encoding.ConsensusVoteTag))
	require.NoError(t, err)

	expectedSig := signature.EncodeSingleSig(hotstuff.SigTypeStaking, stakingSig)

	// check the signature only has staking sig
	require.Equal(t, expectedSig, proposal.SigData)
}
