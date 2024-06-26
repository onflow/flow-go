package signature

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/utils/unittest"
)

func newPacker(identities flow.IdentitySkeletonList) *ConsensusSigDataPacker {
	// mock consensus committee
	committee := &mocks.DynamicCommittee{}
	committee.On("IdentitiesByEpoch", mock.Anything).Return(
		func(_ uint64) flow.IdentitySkeletonList {
			return identities
		},
		nil,
	)

	return NewConsensusSigDataPacker(committee)
}

func makeBlockSigData(committee flow.IdentitySkeletonList) *hotstuff.BlockSignatureData {
	blockSigData := &hotstuff.BlockSignatureData{
		StakingSigners: []flow.Identifier{
			committee[0].NodeID, // A
			committee[2].NodeID, // C
		},
		RandomBeaconSigners: []flow.Identifier{
			committee[3].NodeID, // D
			committee[5].NodeID, // F
		},
		AggregatedStakingSig:         unittest.SignatureFixture(),
		AggregatedRandomBeaconSig:    unittest.SignatureFixture(),
		ReconstructedRandomBeaconSig: unittest.SignatureFixture(),
	}
	return blockSigData
}

// test that a packed data can be unpacked
// given the consensus committee [A, B, C, D, E, F]
// [B,D,F] are random beacon nodes
// [A,C,E] are non-random beacon nodes
// aggregated staking sigs are from [A,C]
// aggregated random beacon sigs are from [D,F]
func TestPackUnpack(t *testing.T) {
	// prepare data for testing
	committee := unittest.IdentityListFixture(6, unittest.WithRole(flow.RoleConsensus)).Sort(flow.Canonical[flow.Identity]).ToSkeleton()
	view := rand.Uint64()
	blockSigData := makeBlockSigData(committee)

	// create packer with the committee
	packer := newPacker(committee)

	// pack & unpack
	signerIndices, sig, err := packer.Pack(view, blockSigData)
	require.NoError(t, err)

	signers, err := signature.DecodeSignerIndicesToIdentities(committee, signerIndices)
	require.NoError(t, err)

	unpacked, err := packer.Unpack(signers, sig)
	require.NoError(t, err)

	// check that the unpacked data match with the original data
	require.Equal(t, blockSigData.StakingSigners, unpacked.StakingSigners)
	require.Equal(t, blockSigData.RandomBeaconSigners, unpacked.RandomBeaconSigners)
	require.Equal(t, blockSigData.AggregatedStakingSig, unpacked.AggregatedStakingSig)
	require.Equal(t, blockSigData.AggregatedRandomBeaconSig, unpacked.AggregatedRandomBeaconSig)
	require.Equal(t, blockSigData.ReconstructedRandomBeaconSig, unpacked.ReconstructedRandomBeaconSig)

	// check the packed signer IDs
	var expectedSignerIDs flow.IdentifierList
	expectedSignerIDs = append(expectedSignerIDs, blockSigData.StakingSigners...)
	expectedSignerIDs = append(expectedSignerIDs, blockSigData.RandomBeaconSigners...)
	require.Equal(t, expectedSignerIDs, signers.NodeIDs())
}

// TestUnpack_EmptySignerList verifies that `Unpack` gracefully handles the edge case
// of an empty signer list, as such could be an input from a byzantine node.
func TestPackUnpack_EmptySigners(t *testing.T) {
	// encode SignatureData with empty SigType vector (this could be an input from a byzantine node)
	byzantineInput := model.SignatureData{
		SigType:                      []byte{},
		AggregatedStakingSig:         unittest.SignatureFixture(),
		AggregatedRandomBeaconSig:    unittest.SignatureFixture(),
		ReconstructedRandomBeaconSig: unittest.SignatureFixture(),
	}
	encoder := new(model.SigDataPacker)
	sig, err := encoder.Encode(&byzantineInput)
	require.NoError(t, err)

	// create packer with a non-empty committee (honest node trying to decode the sig data)
	committee := unittest.IdentityListFixture(6, unittest.WithRole(flow.RoleConsensus)).ToSkeleton()
	packer := newPacker(committee)
	unpacked, err := packer.Unpack(make(flow.IdentitySkeletonList, 0), sig)
	require.NoError(t, err)

	// check that the unpack data match with the original data
	require.Empty(t, unpacked.StakingSigners)
	require.Empty(t, unpacked.RandomBeaconSigners)
	require.Equal(t, byzantineInput.AggregatedStakingSig, unpacked.AggregatedStakingSig)
	require.Equal(t, byzantineInput.AggregatedRandomBeaconSig, unpacked.AggregatedRandomBeaconSig)
	require.Equal(t, byzantineInput.ReconstructedRandomBeaconSig, unpacked.ReconstructedRandomBeaconSig)
}

// if signed by 60 staking nodes, and 50 random beacon nodes among a 200 nodes committee,
// it's able to pack and unpack
func TestPackUnpackManyNodes(t *testing.T) {
	// prepare data for testing
	committee := unittest.IdentityListFixture(200, unittest.WithRole(flow.RoleConsensus)).ToSkeleton()
	view := rand.Uint64()
	blockSigData := makeBlockSigData(committee)
	stakingSigners := make([]flow.Identifier, 0)
	for i := 0; i < 60; i++ {
		stakingSigners = append(stakingSigners, committee[i].NodeID)
	}
	randomBeaconSigners := make([]flow.Identifier, 0)
	for i := 100; i < 100+50; i++ {
		randomBeaconSigners = append(randomBeaconSigners, committee[i].NodeID)
	}
	blockSigData.StakingSigners = stakingSigners
	blockSigData.RandomBeaconSigners = randomBeaconSigners

	// create packer with the committee
	packer := newPacker(committee)

	// pack & unpack
	signerIndices, sig, err := packer.Pack(view, blockSigData)
	require.NoError(t, err)

	signers, err := signature.DecodeSignerIndicesToIdentities(committee, signerIndices)
	require.NoError(t, err)

	unpacked, err := packer.Unpack(signers, sig)
	require.NoError(t, err)

	// check that the unpack data match with the original data
	require.Equal(t, blockSigData.StakingSigners, unpacked.StakingSigners)
	require.Equal(t, blockSigData.RandomBeaconSigners, unpacked.RandomBeaconSigners)
	require.Equal(t, blockSigData.AggregatedStakingSig, unpacked.AggregatedStakingSig)
	require.Equal(t, blockSigData.AggregatedRandomBeaconSig, unpacked.AggregatedRandomBeaconSig)
	require.Equal(t, blockSigData.ReconstructedRandomBeaconSig, unpacked.ReconstructedRandomBeaconSig)

	// check the packed signer IDs
	var expectedSignerIDs flow.IdentifierList
	expectedSignerIDs = append(expectedSignerIDs, blockSigData.StakingSigners...)
	expectedSignerIDs = append(expectedSignerIDs, blockSigData.RandomBeaconSigners...)
	require.Equal(t, expectedSignerIDs, signers.NodeIDs())
}

// if the sig data can not be decoded, return model.InvalidFormatError
func TestFailToDecode(t *testing.T) {
	// prepare data for testing
	committee := unittest.IdentityListFixture(6, unittest.WithRole(flow.RoleConsensus)).ToSkeleton()
	view := rand.Uint64()
	blockSigData := makeBlockSigData(committee)

	// create packer with the committee
	packer := newPacker(committee)

	signerIndices, sig, err := packer.Pack(view, blockSigData)
	require.NoError(t, err)

	signers, err := signature.DecodeSignerIndicesToIdentities(committee, signerIndices)
	require.NoError(t, err)

	// prepare invalid data by modifying the valid data and unpack:
	invalidSigData := sig[1:]
	_, err = packer.Unpack(signers, invalidSigData)
	require.True(t, model.IsInvalidFormatError(err))
}

// TestMismatchSignerIDs
// if the signer IDs doesn't match, return InvalidFormatError
func TestMismatchSignerIDs(t *testing.T) {
	// prepare data for testing
	committee := unittest.IdentityListFixture(9, unittest.WithRole(flow.RoleConsensus)).ToSkeleton()
	view := rand.Uint64()
	blockSigData := makeBlockSigData(committee[:6])

	// create packer with the committee
	packer := newPacker(committee)

	signerIndices, sig, err := packer.Pack(view, blockSigData)
	require.NoError(t, err)

	signers, err := signature.DecodeSignerIndicesToIdentities(committee, signerIndices)
	require.NoError(t, err)

	// prepare invalid signers by modifying the valid signers
	// remove the first signer
	invalidSignerIDs := signers[1:]

	_, err = packer.Unpack(invalidSignerIDs, sig)
	require.True(t, model.IsInvalidFormatError(err))

	// with additional signer
	// 9 nodes committee would require two bytes for sig type, the additional byte
	// would cause the sig type and signer IDs to be mismatch
	invalidSignerIDs = committee
	misPacked, err := packer.Unpack(invalidSignerIDs, sig)
	require.Error(t, err, fmt.Sprintf("packed signers: %v", misPacked))
	require.True(t, model.IsInvalidFormatError(err))
}

// if sig type doesn't match, return InvalidFormatError
func TestInvalidSigType(t *testing.T) {
	// prepare data for testing
	committee := unittest.IdentityListFixture(6, unittest.WithRole(flow.RoleConsensus)).ToSkeleton()
	view := rand.Uint64()
	blockSigData := makeBlockSigData(committee)

	// create packer with the committee
	packer := newPacker(committee)

	signerIndices, sig, err := packer.Pack(view, blockSigData)
	require.NoError(t, err)

	signers, err := signature.DecodeSignerIndicesToIdentities(committee, signerIndices)
	require.NoError(t, err)

	data, err := packer.Decode(sig)
	require.NoError(t, err)

	data.SigType = []byte{1}

	encoded, err := packer.Encode(data)
	require.NoError(t, err)

	_, err = packer.Unpack(signers, encoded)
	require.True(t, model.IsInvalidFormatError(err))
}

// TestPackUnpackWithoutRBAggregatedSig test that a packed data without random beacon signers and
// aggregated random beacon sig can be correctly packed and unpacked
// given the consensus committee [A, B, C]
// [A, B, C] are non-random beacon nodes
// aggregated staking sigs are from [A,B,C]
// no aggregated random beacon sigs
// no random beacon signers
func TestPackUnpackWithoutRBAggregatedSig(t *testing.T) {
	// prepare data for testing
	committee := unittest.IdentityListFixture(3, unittest.WithRole(flow.RoleConsensus)).ToSkeleton()
	view := rand.Uint64()

	blockSigData := &hotstuff.BlockSignatureData{
		StakingSigners:               committee.NodeIDs(),
		RandomBeaconSigners:          nil,
		AggregatedStakingSig:         unittest.SignatureFixture(),
		AggregatedRandomBeaconSig:    nil,
		ReconstructedRandomBeaconSig: unittest.SignatureFixture(),
	}

	// create packer with the committee
	packer := newPacker(committee)

	// pack & unpack
	signerIndices, sig, err := packer.Pack(view, blockSigData)
	require.NoError(t, err)

	signers, err := signature.DecodeSignerIndicesToIdentities(committee, signerIndices)
	require.NoError(t, err)

	unpacked, err := packer.Unpack(signers, sig)
	require.NoError(t, err)

	// check that the unpack data match with the original data
	require.Equal(t, blockSigData.StakingSigners, unpacked.StakingSigners)
	require.Equal(t, blockSigData.AggregatedStakingSig, unpacked.AggregatedStakingSig)
	require.Equal(t, blockSigData.ReconstructedRandomBeaconSig, unpacked.ReconstructedRandomBeaconSig)

	// we need to specifically test if it's empty, it has to be by test definition
	require.Empty(t, unpacked.RandomBeaconSigners)
	require.Empty(t, unpacked.AggregatedRandomBeaconSig)

	// check the packed signer IDs
	expectedSignerIDs := append(flow.IdentifierList{}, blockSigData.StakingSigners...)
	require.Equal(t, expectedSignerIDs, signers.NodeIDs())
}

// TestPackWithoutRBAggregatedSig tests that packer correctly handles BlockSignatureData
// with different structure format, more specifically there is no difference between
// nil and empty slices for RandomBeaconSigners and AggregatedRandomBeaconSig.
func TestPackWithoutRBAggregatedSig(t *testing.T) {
	identities := unittest.IdentityListFixture(3, unittest.WithRole(flow.RoleConsensus)).ToSkeleton()
	committee := identities.NodeIDs()

	// prepare data for testing
	view := rand.Uint64()

	aggregatedSig := unittest.SignatureFixture()
	reconstructedSig := unittest.SignatureFixture()

	blockSigDataWithEmptySlices := &hotstuff.BlockSignatureData{
		StakingSigners:               committee,
		RandomBeaconSigners:          []flow.Identifier{},
		AggregatedStakingSig:         aggregatedSig,
		AggregatedRandomBeaconSig:    []byte{},
		ReconstructedRandomBeaconSig: reconstructedSig,
	}

	blockSigDataWithNils := &hotstuff.BlockSignatureData{
		StakingSigners:               committee,
		RandomBeaconSigners:          nil,
		AggregatedStakingSig:         aggregatedSig,
		AggregatedRandomBeaconSig:    nil,
		ReconstructedRandomBeaconSig: reconstructedSig,
	}

	// create packer with the committee
	packer := newPacker(identities)

	// pack
	signerIDs_A, sig_A, err := packer.Pack(view, blockSigDataWithEmptySlices)
	require.NoError(t, err)

	signerIDs_B, sig_B, err := packer.Pack(view, blockSigDataWithNils)
	require.NoError(t, err)

	// should be the same
	require.Equal(t, signerIDs_A, signerIDs_B)
	require.Equal(t, sig_A, sig_B)
}
