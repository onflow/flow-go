package verification

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/crypto"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/local"
	modulemock "github.com/onflow/flow-go/module/mock"
	msig "github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/utils/unittest"
)

// Test that when beacon key is available for a view, a signed block can pass the validation
// the sig is a random beacon sig.
func TestCombinedSignWithBeaconKeyV3(t *testing.T) {
	// prepare data
	beaconKey := unittest.RandomBeaconPriv()
	pk := beaconKey.PublicKey()
	view := uint64(20)

	fblock := unittest.BlockFixture()
	fblock.Header.View = view
	block := model.BlockFromFlow(fblock.Header)
	signerID := fblock.Header.ProposerID

	beaconKeyStore := modulemock.NewRandomBeaconKeyStore(t)
	beaconKeyStore.On("ByView", view).Return(beaconKey, nil)

	stakingPriv := unittest.StakingPrivKeyFixture()
	nodeID := &unittest.IdentityFixture().IdentitySkeleton
	nodeID.NodeID = signerID
	nodeID.StakingPubKey = stakingPriv.PublicKey()

	me, err := local.New(*nodeID, stakingPriv)
	require.NoError(t, err)
	signer := NewCombinedSignerV3(me, beaconKeyStore)

	dkg := &mocks.DKG{}
	dkg.On("KeyShare", signerID).Return(pk, nil)

	committee := &mocks.DynamicCommittee{}
	committee.On("DKG", mock.Anything).Return(dkg, nil)

	packer := signature.NewConsensusSigDataPacker(committee)
	verifier := NewCombinedVerifierV3(committee, packer)

	// check that a created proposal can be verified by a verifier
	proposal, err := signer.CreateProposal(block)
	require.NoError(t, err)

	vote := proposal.ProposerVote()
	err = verifier.VerifyVote(nodeID, vote.SigData, proposal.Block.View, proposal.Block.BlockID)
	require.NoError(t, err)

	// check that a created proposal's signature is a combined staking sig and random beacon sig
	msg := MakeVoteMessage(block.View, block.BlockID)

	beaconSig, err := beaconKey.Sign(msg, msig.NewBLSHasher(msig.RandomBeaconTag))
	require.NoError(t, err)

	expectedSig := msig.EncodeSingleSig(encoding.SigTypeRandomBeacon, beaconSig)
	require.Equal(t, expectedSig, proposal.SigData)

	// Vote from a node that is _not_ part of the Random Beacon committee should be rejected.
	// Specifically, we expect that the verifier recognizes the `protocol.IdentityNotFoundError`
	// as a sign of an invalid vote and wraps it into a `model.InvalidSignerError`.
	*dkg = mocks.DKG{} // overwrite DKG mock with a new one
	dkg.On("KeyShare", signerID).Return(nil, protocol.IdentityNotFoundError{NodeID: signerID})
	err = verifier.VerifyVote(nodeID, vote.SigData, proposal.Block.View, proposal.Block.BlockID)
	require.True(t, model.IsInvalidSignerError(err))
}

// Test that when beacon key is not available for a view, a signed block can pass the validation
// the sig is a staking sig
func TestCombinedSignWithNoBeaconKeyV3(t *testing.T) {
	// prepare data
	beaconKey := unittest.RandomBeaconPriv()
	pk := beaconKey.PublicKey()
	view := uint64(20)

	fblock := unittest.BlockFixture()
	fblock.Header.View = view
	block := model.BlockFromFlow(fblock.Header)
	signerID := fblock.Header.ProposerID

	beaconKeyStore := modulemock.NewRandomBeaconKeyStore(t)
	beaconKeyStore.On("ByView", view).Return(nil, module.ErrNoBeaconKeyForEpoch)

	stakingPriv := unittest.StakingPrivKeyFixture()
	nodeID := &unittest.IdentityFixture().IdentitySkeleton
	nodeID.NodeID = signerID
	nodeID.StakingPubKey = stakingPriv.PublicKey()

	me, err := local.New(*nodeID, stakingPriv)
	require.NoError(t, err)
	signer := NewCombinedSignerV3(me, beaconKeyStore)

	dkg := &mocks.DKG{}
	dkg.On("KeyShare", signerID).Return(pk, nil)

	committee := &mocks.DynamicCommittee{}
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
	err = verifier.VerifyVote(nodeID, vote.SigData, proposal.Block.View, proposal.Block.BlockID)
	require.NoError(t, err)

	// check that a created proposal's signature is a combined staking sig and random beacon sig
	msg := MakeVoteMessage(block.View, block.BlockID)
	stakingSig, err := stakingPriv.Sign(msg, msig.NewBLSHasher(msig.ConsensusVoteTag))
	require.NoError(t, err)

	expectedSig := msig.EncodeSingleSig(encoding.SigTypeStaking, stakingSig)

	// check the signature only has staking sig
	require.Equal(t, expectedSig, proposal.SigData)
}

// Test_VerifyQC checks that a QC where either signer list is empty is rejected as invalid
func Test_VerifyQCV3(t *testing.T) {
	header := unittest.BlockHeaderFixture()
	block := model.BlockFromFlow(header)
	msg := MakeVoteMessage(block.View, block.BlockID)

	// generate some BLS key as a stub of the random beacon group key and use it to generate a reconstructed beacon sig
	privGroupKey, beaconSig := generateSignature(t, msg, msig.RandomBeaconTag)
	dkg := &mocks.DKG{}
	dkg.On("GroupKey").Return(privGroupKey.PublicKey(), nil)
	dkg.On("Size").Return(uint(20))
	committee := &mocks.DynamicCommittee{}
	committee.On("DKG", mock.Anything).Return(dkg, nil)

	// generate 17 BLS keys as stubs for staking keys and use them to generate an aggregated staking sig
	privStakingKeys, aggStakingSig := generateAggregatedSignature(t, 17, msg, msig.ConsensusVoteTag)
	// generate 11 BLS keys as stubs for individual random beacon key shares and use them to generate an aggregated rand beacon sig
	privRbKeyShares, aggRbSig := generateAggregatedSignature(t, 11, msg, msig.RandomBeaconTag)

	stakingSigners := generateIdentitiesForPrivateKeys(t, privStakingKeys)
	rbSigners := generateIdentitiesForPrivateKeys(t, privRbKeyShares)
	registerPublicRbKeys(t, dkg, rbSigners.NodeIDs(), privRbKeyShares)
	allSigners := append(append(flow.IdentityList{}, stakingSigners...), rbSigners...).ToSkeleton()

	packedSigData := unittest.RandomBytes(1021)
	unpackedSigData := hotstuff.BlockSignatureData{
		StakingSigners:               stakingSigners.NodeIDs(),
		AggregatedStakingSig:         aggStakingSig,
		RandomBeaconSigners:          rbSigners.NodeIDs(),
		AggregatedRandomBeaconSig:    aggRbSig,
		ReconstructedRandomBeaconSig: beaconSig,
	}

	// first, we check that our testing setup works for a correct QC
	t.Run("valid QC", func(t *testing.T) {
		packer := &mocks.Packer{}
		packer.On("Unpack", mock.Anything, packedSigData).Return(&unpackedSigData, nil)

		verifier := NewCombinedVerifierV3(committee, packer)
		err := verifier.VerifyQC(allSigners, packedSigData, block.View, block.BlockID)
		require.NoError(t, err)
	})

	// Here, we test correct verification of a QC, where all replicas signed with their
	// random beacon keys. This is optimal happy path.
	//  * empty list of staking signers
	//  * _no_ aggregated staking sig in QC
	// The Verifier should accept such QC
	t.Run("all replicas signed with random beacon keys", func(t *testing.T) {
		sd := unpackedSigData // copy correct QC
		sd.StakingSigners = []flow.Identifier{}
		sd.AggregatedStakingSig = []byte{}

		packer := &mocks.Packer{}
		packer.On("Unpack", mock.Anything, packedSigData).Return(&sd, nil)
		verifier := NewCombinedVerifierV3(committee, packer)
		err := verifier.VerifyQC(allSigners, packedSigData, block.View, block.BlockID)
		require.NoError(t, err)
	})

	// Modify the correct QC:
	//  * empty list of staking signers
	//  * but an aggregated staking sig is given
	// The Verifier should recognize this as an invalid QC.
	t.Run("empty staking signers but aggregated staking sig in QC", func(t *testing.T) {
		sd := unpackedSigData // copy correct QC
		sd.StakingSigners = []flow.Identifier{}

		packer := &mocks.Packer{}
		packer.On("Unpack", mock.Anything, packedSigData).Return(&sd, nil)
		verifier := NewCombinedVerifierV3(committee, packer)
		err := verifier.VerifyQC(allSigners, packedSigData, block.View, block.BlockID)
		require.True(t, model.IsInvalidFormatError(err))
	})

	// Modify the correct QC: empty list of random beacon signers.
	// The Verifier should recognize this as an invalid QC
	t.Run("empty random beacon signers", func(t *testing.T) {
		sd := unpackedSigData // copy correct QC
		sd.RandomBeaconSigners = []flow.Identifier{}

		packer := &mocks.Packer{}
		packer.On("Unpack", mock.Anything, packedSigData).Return(&sd, nil)
		verifier := NewCombinedVerifierV3(committee, packer)
		err := verifier.VerifyQC(allSigners, packedSigData, block.View, block.BlockID)
		require.True(t, model.IsInvalidFormatError(err))
	})

	// Modify the correct QC: too few random beacon signers.
	// The Verifier should recognize this as an invalid QC
	t.Run("too few random beacon signers", func(t *testing.T) {
		// In total, we have 20 DKG participants, i.e. we require at least 10 random
		// beacon sig shares. But we only supply 5 aggregated key shares.
		sd := unpackedSigData // copy correct QC
		sd.RandomBeaconSigners = rbSigners[:5].NodeIDs()
		sd.AggregatedRandomBeaconSig = aggregatedSignature(t, privRbKeyShares[:5], msg, msig.RandomBeaconTag)

		packer := &mocks.Packer{}
		packer.On("Unpack", mock.Anything, packedSigData).Return(&sd, nil)
		verifier := NewCombinedVerifierV3(committee, packer)
		err := verifier.VerifyQC(allSigners, packedSigData, block.View, block.BlockID)
		require.True(t, model.IsInvalidFormatError(err))
	})

}

// Test_VerifyQC_EmptySignersV3 checks that Verifier returns an `model.InsufficientSignaturesError`
// if `signers` input is empty or nil. This check should happen _before_ the Verifier calls into
// any sub-components, because some (e.g. `crypto.AggregateBLSPublicKeys`) don't provide sufficient
// sentinel errors to distinguish between internal problems and external byzantine inputs.
func Test_VerifyQC_EmptySignersV3(t *testing.T) {
	committee := &mocks.DynamicCommittee{}
	dkg := &mocks.DKG{}
	pk := &modulemock.PublicKey{}
	pk.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
	dkg.On("GroupKey").Return(pk)
	committee.On("DKG", mock.Anything).Return(dkg, nil)
	packer := signature.NewConsensusSigDataPacker(committee)
	verifier := NewCombinedVerifier(committee, packer)

	header := unittest.BlockHeaderFixture()
	block := model.BlockFromFlow(header)
	// sigData with empty signers
	emptySignersInput := model.SignatureData{
		SigType:                      []byte{},
		AggregatedStakingSig:         unittest.SignatureFixture(),
		AggregatedRandomBeaconSig:    unittest.SignatureFixture(),
		ReconstructedRandomBeaconSig: unittest.SignatureFixture(),
	}
	encoder := new(model.SigDataPacker)
	sigData, err := encoder.Encode(&emptySignersInput)
	require.NoError(t, err)

	err = verifier.VerifyQC(flow.IdentitySkeletonList{}, sigData, block.View, block.BlockID)
	require.True(t, model.IsInsufficientSignaturesError(err))

	err = verifier.VerifyQC(nil, sigData, block.View, block.BlockID)
	require.True(t, model.IsInsufficientSignaturesError(err))
}

// TestCombinedSign_BeaconKeyStore_ViewForUnknownEpochv3 tests that if the beacon
// key store reports the view of the entity to sign has no known epoch, an
// exception should be raised.
func TestCombinedSign_BeaconKeyStore_ViewForUnknownEpochv3(t *testing.T) {
	beaconKeyStore := modulemock.NewRandomBeaconKeyStore(t)
	beaconKeyStore.On("ByView", mock.Anything).Return(nil, model.ErrViewForUnknownEpoch)

	stakingPriv := unittest.StakingPrivKeyFixture()
	nodeID := unittest.IdentityFixture()
	nodeID.StakingPubKey = stakingPriv.PublicKey()

	me, err := local.New(nodeID.IdentitySkeleton, stakingPriv)
	require.NoError(t, err)
	signer := NewCombinedSigner(me, beaconKeyStore)

	fblock := unittest.BlockHeaderFixture()
	block := model.BlockFromFlow(fblock)

	vote, err := signer.CreateVote(block)
	require.Error(t, err)
	assert.Nil(t, vote)
}

func generateIdentitiesForPrivateKeys(t *testing.T, pivKeys []crypto.PrivateKey) flow.IdentityList {
	ids := make([]*flow.Identity, 0, len(pivKeys))
	for _, k := range pivKeys {
		id := unittest.IdentityFixture(
			unittest.WithRole(flow.RoleConsensus),
			unittest.WithStakingPubKey(k.PublicKey()),
		)
		ids = append(ids, id)
	}
	return ids
}

func registerPublicRbKeys(t *testing.T, dkg *mocks.DKG, signerIDs []flow.Identifier, pivKeys []crypto.PrivateKey) {
	assert.Equal(t, len(signerIDs), len(pivKeys), "one signer ID per key expected")
	for k, id := range signerIDs {
		dkg.On("KeyShare", id).Return(pivKeys[k].PublicKey(), nil)
	}
}

// generateAggregatedSignature generates `n` private BLS keys, signs `msg` which each key,
// and aggregates the resulting sigs. Returns private keys and aggregated sig.
func generateAggregatedSignature(t *testing.T, n int, msg []byte, tag string) ([]crypto.PrivateKey, crypto.Signature) {
	sigs := make([]crypto.Signature, 0, n)
	privs := make([]crypto.PrivateKey, 0, n)
	for ; n > 0; n-- {
		priv, sig := generateSignature(t, msg, tag)
		sigs = append(sigs, sig)
		privs = append(privs, priv)
	}
	agg, err := crypto.AggregateBLSSignatures(sigs)
	require.NoError(t, err)
	return privs, agg
}

// generateSignature creates a single private BLS 12-381 key, signs the provided `message` with
// using domain separation `tag` and return the private key and signature.
func generateSignature(t *testing.T, message []byte, tag string) (crypto.PrivateKey, crypto.Signature) {
	priv := unittest.PrivateKeyFixture(crypto.BLSBLS12381, crypto.KeyGenSeedMinLen)
	sig, err := priv.Sign(message, msig.NewBLSHasher(tag))
	require.NoError(t, err)
	return priv, sig
}

func aggregatedSignature(t *testing.T, pivKeys []crypto.PrivateKey, message []byte, tag string) crypto.Signature {
	hasher := msig.NewBLSHasher(tag)
	sigs := make([]crypto.Signature, 0, len(pivKeys))
	for _, k := range pivKeys {
		sig, err := k.Sign(message, hasher)
		require.NoError(t, err)
		sigs = append(sigs, sig)
	}
	agg, err := crypto.AggregateBLSSignatures(sigs)
	require.NoError(t, err)
	return agg
}
