package signature

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/utils/unittest"
)

func newPacker(identities flow.IdentityList) *ConsensusSigDataPacker {
	// mock consensus committee
	committee := &mocks.Committee{}
	committee.On("Identities", mock.Anything, mock.Anything).Return(
		func(blockID flow.Identifier, selector flow.IdentityFilter) flow.IdentityList {
			return identities.Filter(selector)
		},
		nil,
	)

	return NewConsensusSigDataPacker(committee)
}

func makeBlockSigData(committee []flow.Identifier) *hotstuff.BlockSignatureData {
	blockSigData := &hotstuff.BlockSignatureData{
		StakingSigners: []flow.Identifier{
			committee[0], // A
			committee[2], // C
		},
		RandomBeaconSigners: []flow.Identifier{
			committee[3], // D
			committee[5], // F
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
	identities := unittest.IdentityListFixture(6, unittest.WithRole(flow.RoleConsensus))
	committee := identities.NodeIDs()

	// prepare data for testing
	blockID := unittest.IdentifierFixture()
	blockSigData := makeBlockSigData(committee)

	// create packer with the committee
	packer := newPacker(identities)

	// pack & unpack
	signerIDs, sig, err := packer.Pack(blockID, blockSigData)
	require.NoError(t, err)

	unpacked, err := packer.Unpack(blockID, signerIDs, sig)
	require.NoError(t, err)

	// check that the unpack data match with the original data
	require.Equal(t, blockSigData.StakingSigners, unpacked.StakingSigners)
	require.Equal(t, blockSigData.RandomBeaconSigners, unpacked.RandomBeaconSigners)
	require.Equal(t, blockSigData.AggregatedStakingSig, unpacked.AggregatedStakingSig)
	require.Equal(t, blockSigData.AggregatedRandomBeaconSig, unpacked.AggregatedRandomBeaconSig)
	require.Equal(t, blockSigData.ReconstructedRandomBeaconSig, unpacked.ReconstructedRandomBeaconSig)

	// check the packed signer IDs
	expectedSignerIDs := []flow.Identifier{}
	expectedSignerIDs = append(expectedSignerIDs, blockSigData.StakingSigners...)
	expectedSignerIDs = append(expectedSignerIDs, blockSigData.RandomBeaconSigners...)
	require.Equal(t, expectedSignerIDs, signerIDs)
}

// if signed by 60 staking nodes, and 50 random beacon nodes among a 200 nodes committee,
// it's able to pack and unpack
func TestPackUnpackManyNodes(t *testing.T) {
	identities := unittest.IdentityListFixture(200, unittest.WithRole(flow.RoleConsensus))
	committee := identities.NodeIDs()

	// prepare data for testing
	blockID := unittest.IdentifierFixture()
	blockSigData := makeBlockSigData(committee)
	stakingSigners := make([]flow.Identifier, 0)
	for i := 0; i < 60; i++ {
		stakingSigners = append(stakingSigners, committee[i])
	}
	randomBeaconSigners := make([]flow.Identifier, 0)
	for i := 100; i < 100+50; i++ {
		randomBeaconSigners = append(randomBeaconSigners, committee[i])
	}
	blockSigData.StakingSigners = stakingSigners
	blockSigData.RandomBeaconSigners = randomBeaconSigners

	// create packer with the committee
	packer := newPacker(identities)

	// pack & unpack
	signerIDs, sig, err := packer.Pack(blockID, blockSigData)
	require.NoError(t, err)

	unpacked, err := packer.Unpack(blockID, signerIDs, sig)
	require.NoError(t, err)

	// check that the unpack data match with the original data
	require.Equal(t, blockSigData.StakingSigners, unpacked.StakingSigners)
	require.Equal(t, blockSigData.RandomBeaconSigners, unpacked.RandomBeaconSigners)
	require.Equal(t, blockSigData.AggregatedStakingSig, unpacked.AggregatedStakingSig)
	require.Equal(t, blockSigData.AggregatedRandomBeaconSig, unpacked.AggregatedRandomBeaconSig)
	require.Equal(t, blockSigData.ReconstructedRandomBeaconSig, unpacked.ReconstructedRandomBeaconSig)

	// check the packed signer IDs
	expectedSignerIDs := []flow.Identifier{}
	expectedSignerIDs = append(expectedSignerIDs, blockSigData.StakingSigners...)
	expectedSignerIDs = append(expectedSignerIDs, blockSigData.RandomBeaconSigners...)
	require.Equal(t, expectedSignerIDs, signerIDs)
}

// if the sig data can not be decoded, return ErrInvalidFormat
func TestFailToDecode(t *testing.T) {
	identities := unittest.IdentityListFixture(6, unittest.WithRole(flow.RoleConsensus))
	committee := identities.NodeIDs()

	// prepare data for testing
	blockID := unittest.IdentifierFixture()
	blockSigData := makeBlockSigData(committee)

	// create packer with the committee
	packer := newPacker(identities)

	signerIDs, sig, err := packer.Pack(blockID, blockSigData)
	require.NoError(t, err)

	// prepare invalid data by modifying the valid data
	invalidSigData := sig[1:]

	_, err = packer.Unpack(blockID, signerIDs, invalidSigData)

	require.Error(t, err)
	require.True(t, errors.Is(err, signature.ErrInvalidFormat))
}

// if the signer IDs doesn't match, return InvalidFormatError
func TestMismatchSignerIDs(t *testing.T) {
	identities := unittest.IdentityListFixture(6, unittest.WithRole(flow.RoleConsensus))
	committee := identities.NodeIDs()

	// prepare data for testing
	blockID := unittest.IdentifierFixture()
	blockSigData := makeBlockSigData(committee)

	// create packer with the committee
	packer := newPacker(identities)

	signerIDs, sig, err := packer.Pack(blockID, blockSigData)
	require.NoError(t, err)

	// prepare invalid signerIDs by modifying the valid signerIDs
	// remove the first signer
	invalidSignerIDs := signerIDs[1:]

	_, err = packer.Unpack(blockID, invalidSignerIDs, sig)

	require.Error(t, err)
	require.True(t, errors.Is(err, signature.ErrInvalidFormat))

	// prepare invalid signerIDs by modifying the valid signerIDs
	// adding one more signer
	invalidSignerIDs = append(signerIDs, unittest.IdentifierFixture())
	misPacked, err := packer.Unpack(blockID, invalidSignerIDs, sig)

	require.Error(t, err, fmt.Sprintf("packed signers: %v", misPacked))
	require.True(t, errors.Is(err, signature.ErrInvalidFormat))
}

// if sig type doesn't match, return InvalidFormatError
func TestInvalidSigType(t *testing.T) {
	identities := unittest.IdentityListFixture(6, unittest.WithRole(flow.RoleConsensus))
	committee := identities.NodeIDs()

	// prepare data for testing
	blockID := unittest.IdentifierFixture()
	blockSigData := makeBlockSigData(committee)

	// create packer with the committee
	packer := newPacker(identities)

	signerIDs, sig, err := packer.Pack(blockID, blockSigData)
	require.NoError(t, err)

	var data signatureData
	err = packer.encoder.Decode(sig, &data)
	require.NoError(t, err)

	data.SigType = []byte{1}

	encoded, err := packer.encoder.Encode(data)
	require.NoError(t, err)

	_, err = packer.Unpack(blockID, signerIDs, encoded)

	require.True(t, errors.Is(err, signature.ErrInvalidFormat))
}

func TestSerializeAndDeserializeSigTypes(t *testing.T) {
	t.Run("nothing", func(t *testing.T) {
		expected := []hotstuff.SigType{}
		bytes, err := serializeToBitVector(expected)
		require.NoError(t, err)
		require.Equal(t, []byte{}, bytes)

		types, err := deserializeFromBytes(bytes, len(expected))
		require.NoError(t, err)
		require.Equal(t, expected, types)
	})

	t.Run("1 SigTypeStaking", func(t *testing.T) {
		expected := []hotstuff.SigType{hotstuff.SigTypeStaking}
		bytes, err := serializeToBitVector(expected)
		require.NoError(t, err)
		require.Equal(t, []byte{0}, bytes)

		types, err := deserializeFromBytes(bytes, len(expected))
		require.NoError(t, err)
		require.Equal(t, expected, types)
	})

	t.Run("1 SigTypeRandomBeacon", func(t *testing.T) {
		expected := []hotstuff.SigType{hotstuff.SigTypeRandomBeacon}
		bytes, err := serializeToBitVector(expected)
		require.NoError(t, err)
		require.Equal(t, []byte{1 << 7}, bytes)

		types, err := deserializeFromBytes(bytes, len(expected))
		require.NoError(t, err)
		require.Equal(t, expected, types)
	})

	t.Run("2 SigTypeRandomBeacon", func(t *testing.T) {
		expected := []hotstuff.SigType{
			hotstuff.SigTypeRandomBeacon,
			hotstuff.SigTypeRandomBeacon,
		}
		bytes, err := serializeToBitVector(expected)
		require.NoError(t, err)
		require.Equal(t, []byte{1<<7 + 1<<6}, bytes)

		types, err := deserializeFromBytes(bytes, len(expected))
		require.NoError(t, err)
		require.Equal(t, expected, types)
	})

	t.Run("8 SigTypeRandomBeacon", func(t *testing.T) {
		count := 8
		expected := make([]hotstuff.SigType, 0)
		for i := 0; i < count; i++ {
			expected = append(expected, hotstuff.SigTypeRandomBeacon)
		}
		bytes, err := serializeToBitVector(expected)
		require.NoError(t, err)
		require.Equal(t, []byte{255}, bytes)

		types, err := deserializeFromBytes(bytes, len(expected))
		require.NoError(t, err)
		require.Equal(t, expected, types)
	})

	t.Run("8 SigTypeStaking", func(t *testing.T) {
		count := 8
		expected := make([]hotstuff.SigType, 0)
		for i := 0; i < count; i++ {
			expected = append(expected, hotstuff.SigTypeStaking)
		}
		bytes, err := serializeToBitVector(expected)
		require.NoError(t, err)
		require.Equal(t, []byte{0}, bytes)

		types, err := deserializeFromBytes(bytes, len(expected))
		require.NoError(t, err)
		require.Equal(t, expected, types)
	})

	t.Run("9 SigTypeRandomBeacon", func(t *testing.T) {
		count := 9
		expected := make([]hotstuff.SigType, 0)
		for i := 0; i < count; i++ {
			expected = append(expected, hotstuff.SigTypeRandomBeacon)
		}
		bytes, err := serializeToBitVector(expected)
		require.NoError(t, err)
		require.Equal(t, []byte{255, 1 << 7}, bytes)

		types, err := deserializeFromBytes(bytes, len(expected))
		require.NoError(t, err)
		require.Equal(t, expected, types)
	})

	t.Run("9 SigTypeStaking", func(t *testing.T) {
		count := 9
		expected := make([]hotstuff.SigType, 0)
		for i := 0; i < count; i++ {
			expected = append(expected, hotstuff.SigTypeStaking)
		}
		bytes, err := serializeToBitVector(expected)
		require.NoError(t, err)
		require.Equal(t, []byte{0, 0}, bytes)

		types, err := deserializeFromBytes(bytes, len(expected))
		require.NoError(t, err)
		require.Equal(t, expected, types)
	})

	t.Run("16 SigTypeRandomBeacon, 2 groups", func(t *testing.T) {
		count := 16
		expected := make([]hotstuff.SigType, 0)
		for i := 0; i < count; i++ {
			expected = append(expected, hotstuff.SigTypeRandomBeacon)
		}
		bytes, err := serializeToBitVector(expected)
		require.NoError(t, err)
		require.Equal(t, []byte{255, 255}, bytes)

		types, err := deserializeFromBytes(bytes, len(expected))
		require.NoError(t, err)
		require.Equal(t, expected, types)
	})

	t.Run("3 SigTypeRandomBeacon, 4 SigTypeStaking", func(t *testing.T) {
		random, staking := 3, 4
		expected := make([]hotstuff.SigType, 0)
		for i := 0; i < random; i++ {
			expected = append(expected, hotstuff.SigTypeRandomBeacon)
		}
		for i := 0; i < staking; i++ {
			expected = append(expected, hotstuff.SigTypeStaking)
		}
		bytes, err := serializeToBitVector(expected)
		require.NoError(t, err)
		require.Equal(t, []byte{1<<7 + 1<<6 + 1<<5}, bytes)

		types, err := deserializeFromBytes(bytes, len(expected))
		require.NoError(t, err)
		require.Equal(t, expected, types)
	})

	t.Run("3 SigTypeStaking, 4 SigTypeRandomBeacon", func(t *testing.T) {
		staking, random := 3, 4
		expected := make([]hotstuff.SigType, 0)
		for i := 0; i < staking; i++ {
			expected = append(expected, hotstuff.SigTypeStaking)
		}
		for i := 0; i < random; i++ {
			expected = append(expected, hotstuff.SigTypeRandomBeacon)
		}
		bytes, err := serializeToBitVector(expected)
		require.NoError(t, err)
		// 00011110
		require.Equal(t, []byte{1<<4 + 1<<3 + 1<<2 + 1<<1}, bytes)

		types, err := deserializeFromBytes(bytes, len(expected))
		require.NoError(t, err)
		require.Equal(t, expected, types)
	})

	t.Run("3 SigTypeStaking, 6 SigTypeRandomBeacon", func(t *testing.T) {
		staking, random := 3, 6
		expected := make([]hotstuff.SigType, 0)
		for i := 0; i < staking; i++ {
			expected = append(expected, hotstuff.SigTypeStaking)
		}
		for i := 0; i < random; i++ {
			expected = append(expected, hotstuff.SigTypeRandomBeacon)
		}
		bytes, err := serializeToBitVector(expected)
		require.NoError(t, err)
		// 00011110, 10000000
		require.Equal(t, []byte{1<<4 + 1<<3 + 1<<2 + 1<<1 + 1, 1 << 7}, bytes)

		types, err := deserializeFromBytes(bytes, len(expected))
		require.NoError(t, err)
		require.Equal(t, expected, types)
	})
}

func TestDeserializeMismatchingBytes(t *testing.T) {
	count := 9
	expected := make([]hotstuff.SigType, 0)
	for i := 0; i < count; i++ {
		expected = append(expected, hotstuff.SigTypeStaking)
	}
	bytes, err := serializeToBitVector(expected)
	require.NoError(t, err)

	for invalidCount := 0; invalidCount < 100; invalidCount++ {
		if invalidCount >= count && invalidCount <= 16 {
			// skip correct count
			continue
		}
		_, err := deserializeFromBytes(bytes, invalidCount)
		require.Error(t, err, fmt.Sprintf("invalid count: %v", invalidCount))
		require.True(t, errors.Is(err, signature.ErrInvalidFormat), fmt.Sprintf("invalid count: %v", invalidCount))
	}
}

func TestDeserializeInvalidTailingBits(t *testing.T) {
	_, err := deserializeFromBytes([]byte{255, 1<<7 + 1<<1}, 9)
	require.Error(t, err)
	require.True(t, errors.Is(err, signature.ErrInvalidFormat))
	require.Contains(t, fmt.Sprintf("%v", err), "remaining bits")
}

// TestPackUnpackWithoutRBAggregatedSig test that a packed data without random beacon signers and
// aggregated random beacon sig can be correctly packed and unpacked
// given the consensus committee [A, B, C]
// [A, B, C] are non-random beacon nodes
// aggregated staking sigs are from [A,B,C]
// no aggregated random beacon sigs
// no random beacon signers
func TestPackUnpackWithoutRBAggregatedSig(t *testing.T) {
	identities := unittest.IdentityListFixture(3, unittest.WithRole(flow.RoleConsensus))
	committee := identities.NodeIDs()

	// prepare data for testing
	blockID := unittest.IdentifierFixture()

	blockSigData := &hotstuff.BlockSignatureData{
		StakingSigners:               committee,
		RandomBeaconSigners:          nil,
		AggregatedStakingSig:         unittest.SignatureFixture(),
		AggregatedRandomBeaconSig:    nil,
		ReconstructedRandomBeaconSig: unittest.SignatureFixture(),
	}

	// create packer with the committee
	packer := newPacker(identities)

	// pack & unpack
	signerIDs, sig, err := packer.Pack(blockID, blockSigData)
	require.NoError(t, err)

	unpacked, err := packer.Unpack(blockID, signerIDs, sig)
	require.NoError(t, err)

	// check that the unpack data match with the original data
	require.Equal(t, blockSigData.StakingSigners, unpacked.StakingSigners)
	require.Equal(t, blockSigData.AggregatedStakingSig, unpacked.AggregatedStakingSig)
	require.Equal(t, blockSigData.ReconstructedRandomBeaconSig, unpacked.ReconstructedRandomBeaconSig)

	// we need to specifically test if it's empty, it has to be by test definition
	require.Empty(t, unpacked.RandomBeaconSigners)
	require.Empty(t, unpacked.AggregatedRandomBeaconSig)

	// check the packed signer IDs
	expectedSignerIDs := append([]flow.Identifier{}, blockSigData.StakingSigners...)
	require.Equal(t, expectedSignerIDs, signerIDs)
}

// TestPackWithoutRBAggregatedSig tests that packer correctly handles BlockSignatureData
// with different structure format, more specifically there is no difference between
// nil and empty slices for RandomBeaconSigners and AggregatedRandomBeaconSig.
func TestPackWithoutRBAggregatedSig(t *testing.T) {
	identities := unittest.IdentityListFixture(3, unittest.WithRole(flow.RoleConsensus))
	committee := identities.NodeIDs()

	// prepare data for testing
	blockID := unittest.IdentifierFixture()

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
	signerIDs_A, sig_A, err := packer.Pack(blockID, blockSigDataWithEmptySlices)
	require.NoError(t, err)

	signerIDs_B, sig_B, err := packer.Pack(blockID, blockSigDataWithNils)
	require.NoError(t, err)

	// should be the same
	require.Equal(t, signerIDs_A, signerIDs_B)
	require.Equal(t, sig_A, sig_B)
}
