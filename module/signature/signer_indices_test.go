package signature_test

import (
	"fmt"
	"sort"
	"testing"

	"pgregory.net/rapid"

	"github.com/onflow/flow-go/ledger/common/bitutils"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/flow/filter/id"
	"github.com/onflow/flow-go/model/flow/order"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestEncodeDecodeIdentities verifies the two path of encoding -> decoding:
//  1. Identifiers --encode--> Indices --decode--> Identifiers
//  2. for the decoding step, we offer an optimized convenience function to directly
//     decode to full identities: Indices --decode--> Identities
func TestEncodeDecodeIdentities(t *testing.T) {
	canonicalIdentities := unittest.IdentityListFixture(20)
	canonicalIdentifiers := canonicalIdentities.NodeIDs()
	for s := 0; s < 20; s++ {
		for e := s; e < 20; e++ {
			var signers = canonicalIdentities[s:e]

			// encoding
			indices, err := signature.EncodeSignersToIndices(canonicalIdentities.NodeIDs(), signers.NodeIDs())
			require.NoError(t, err)

			// decoding option 1: decode to Identifiers
			decodedIDs, err := signature.DecodeSignerIndicesToIdentifiers(canonicalIdentifiers, indices)
			require.NoError(t, err)
			require.Equal(t, signers.NodeIDs(), decodedIDs)

			// decoding option 1: decode to Identifiers
			decodedIdentities, err := signature.DecodeSignerIndicesToIdentities(canonicalIdentities, indices)
			require.NoError(t, err)
			require.Equal(t, signers, decodedIdentities)
		}
	}
}

// TestEncodeFail verifies that an error is returned in case some signer is not part
// of the set of canonicalIdentifiers
func TestEncodeFail(t *testing.T) {
	fullIdentities := unittest.IdentifierListFixture(20)
	_, err := signature.EncodeSignersToIndices(fullIdentities[1:], fullIdentities[:10])
	require.Error(t, err)
}

// Test_EncodeSignerToIndicesAndSigType uses fuzzy-testing framework Rapid to
// test the method EncodeSignerToIndicesAndSigType:
// * we generate a set of authorized signer: `committeeIdentities`
// * part of this set is sampled as staking singers: `stakingSigners`
// * another part of `committeeIdentities` is sampled as beacon singers: `beaconSigners`
// * we encode the set and check that the results conform to the protocol specification
func Test_EncodeSignerToIndicesAndSigType(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// select total committee size, number of random beacon signers and number of staking signers
		committeeSize := rapid.IntRange(1, 272).Draw(t, "committeeSize").(int)
		numStakingSigners := rapid.IntRange(0, committeeSize).Draw(t, "numStakingSigners").(int)
		numRandomBeaconSigners := rapid.IntRange(0, committeeSize-numStakingSigners).Draw(t, "numRandomBeaconSigners").(int)

		// create committee
		committeeIdentities := unittest.IdentityListFixture(committeeSize, unittest.WithRole(flow.RoleConsensus)).Sort(order.Canonical)
		committee := committeeIdentities.NodeIDs()
		stakingSigners, beaconSigners := sampleSigners(committee, numStakingSigners, numRandomBeaconSigners)

		// encode
		signerIndices, sigTypes, err := signature.EncodeSignerToIndicesAndSigType(committee, stakingSigners, beaconSigners)
		require.NoError(t, err)

		// check verify signer indices
		unorderedSigners := stakingSigners.Union(beaconSigners) // caution, the Union operation potentially changes the ordering
		correctEncoding(t, signerIndices, committee, unorderedSigners)

		// check sigTypes
		canSigners := committeeIdentities.Filter(filter.HasNodeID(unorderedSigners...)).NodeIDs() // generates list of signer IDs in canonical order
		correctEncoding(t, sigTypes, canSigners, beaconSigners)
	})
}

// Test_EncodeSignersToIndices uses fuzzy-testing framework Rapid to test the method EncodeSignersToIndices:
// * we generate a set of authorized signer: `identities`
// * part of this set is sampled as singers: `signers`
// * we encode the set and check that the results conform to the protocol specification
func Test_EncodeSignersToIndices(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// select total committee size, number of random beacon signers and number of staking signers
		committeeSize := rapid.IntRange(1, 272).Draw(t, "committeeSize").(int)
		numSigners := rapid.IntRange(0, committeeSize).Draw(t, "numSigners").(int)

		// create committee
		identities := unittest.IdentityListFixture(committeeSize, unittest.WithRole(flow.RoleConsensus)).Sort(order.Canonical)
		committee := identities.NodeIDs()
		signers := committee.Sample(uint(numSigners))

		// encode
		signerIndices, err := signature.EncodeSignersToIndices(committee, signers)
		require.NoError(t, err)

		// check verify signer indices
		correctEncoding(t, signerIndices, committee, signers)
	})
}

// Test_DecodeSignerIndicesToIdentifiers uses fuzzy-testing framework Rapid to test the method DecodeSignerIndicesToIdentifiers:
// * we generate a set of authorized signer: `identities`
// * part of this set is sampled as singers: `signers`
// * We encode the set using `EncodeSignersToIndices` (tested before) and then decode it.
//   Thereby we should recover the original input. Caution, the order might be different,
//   so we sort both sets.
func Test_DecodeSignerIndicesToIdentifiers(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// select total committee size, number of random beacon signers and number of staking signers
		committeeSize := rapid.IntRange(1, 272).Draw(t, "committeeSize").(int)
		numSigners := rapid.IntRange(0, committeeSize).Draw(t, "numSigners").(int)

		// create committee
		identities := unittest.IdentityListFixture(committeeSize, unittest.WithRole(flow.RoleConsensus)).Sort(order.Canonical)
		committee := identities.NodeIDs()
		signers := committee.Sample(uint(numSigners))
		sort.Sort(signers)

		// encode
		signerIndices, err := signature.EncodeSignersToIndices(committee, signers)
		require.NoError(t, err)

		// decode and verify
		decodedSigners, err := signature.DecodeSignerIndicesToIdentifiers(committee, signerIndices)
		require.NoError(t, err)
		sort.Sort(decodedSigners)
		require.Equal(t, signers, decodedSigners)
	})
}

// Test_DecodeSignerIndicesToIdentities uses fuzzy-testing framework Rapid to test the method DecodeSignerIndicesToIdentities:
// * we generate a set of authorized signer: `identities`
// * part of this set is sampled as singers: `signers`
// * We encode the set using `EncodeSignersToIndices` (tested before) and then decode it.
//   Thereby we should recover the original input. Caution, the order might be different,
//   so we sort both sets.
// Note: this is _almost_ the same test as `Test_DecodeSignerIndicesToIdentifiers`. However, in the other
// test, we decode to node IDs; while in this test, we decode to full _Identities_.
func Test_DecodeSignerIndicesToIdentities(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// select total committee size, number of random beacon signers and number of staking signers
		committeeSize := rapid.IntRange(1, 272).Draw(t, "committeeSize").(int)
		numSigners := rapid.IntRange(0, committeeSize).Draw(t, "numSigners").(int)

		// create committee
		identities := unittest.IdentityListFixture(committeeSize, unittest.WithRole(flow.RoleConsensus)).Sort(order.Canonical)
		signers := identities.Sample(uint(numSigners))

		// encode
		signerIndices, err := signature.EncodeSignersToIndices(identities.NodeIDs(), signers.NodeIDs())
		require.NoError(t, err)

		// decode and verify
		decodedSigners, err := signature.DecodeSignerIndicesToIdentities(identities, signerIndices)
		require.NoError(t, err)
		require.Equal(t, signers.Sort(order.Canonical), decodedSigners.Sort(order.Canonical))
	})
}

//// if the sig data can not be decoded, return model.InvalidFormatError
//func TestFailToDecode(t *testing.T) {
//	identities := unittest.IdentityListFixture(6, unittest.WithRole(flow.RoleConsensus))
//	committee := identities.NodeIDs()
//
//	// prepare data for testing
//	blockID := unittest.IdentifierFixture()
//	blockSigData := makeBlockSigData(committee)
//
//	// create packer with the committee
//	packer := newPacker(identities)
//
//	signerIndices, sig, err := packer.Pack(blockID, blockSigData)
//	require.NoError(t, err)
//
//	signerIDs, err := signature.DecodeSignerIndicesToIdentifiers(committee, signerIndices)
//	require.NoError(t, err)
//
//	// prepare invalid data by modifying the valid data
//	invalidSigData := sig[1:]
//
//	_, err = packer.Unpack(signerIDs, invalidSigData)
//
//	require.True(t, model.IsInvalidFormatError(err))
//}
//
//// if the signer IDs doesn't match, return InvalidFormatError
//func TestMismatchSignerIDs(t *testing.T) {
//	identities := unittest.IdentityListFixture(9, unittest.WithRole(flow.RoleConsensus))
//	committee := identities.NodeIDs()
//
//	// prepare data for testing
//	blockID := unittest.IdentifierFixture()
//	blockSigData := makeBlockSigData(committee[:6])
//
//	// create packer with the committee
//	packer := newPacker(identities)
//
//	signerIndices, sig, err := packer.Pack(blockID, blockSigData)
//	require.NoError(t, err)
//
//	signerIDs, err := signature.DecodeSignerIndicesToIdentifiers(committee, signerIndices)
//	require.NoError(t, err)
//
//	// prepare invalid signerIDs by modifying the valid signerIDs
//	// remove the first signer
//	invalidSignerIDs := signerIDs[1:]
//
//	_, err = packer.Unpack(invalidSignerIDs, sig)
//
//	require.True(t, model.IsInvalidFormatError(err))
//
//	// with additional signer
//	// 9 nodes committee would require two bytes for sig type, the additional byte
//	// would cause the sig type and signer IDs to be mismatch
//	invalidSignerIDs = committee
//	misPacked, err := packer.Unpack(invalidSignerIDs, sig)
//
//	require.Error(t, err, fmt.Sprintf("packed signers: %v", misPacked))
//	require.True(t, model.IsInvalidFormatError(err))
//}
//
//// if sig type doesn't match, return InvalidFormatError
//func TestInvalidSigType(t *testing.T) {
//	identities := unittest.IdentityListFixture(6, unittest.WithRole(flow.RoleConsensus))
//	committee := identities.NodeIDs()
//
//	// prepare data for testing
//	blockID := unittest.IdentifierFixture()
//	blockSigData := makeBlockSigData(committee)
//
//	// create packer with the committee
//	packer := newPacker(identities)
//
//	signerIndices, sig, err := packer.Pack(blockID, blockSigData)
//	require.NoError(t, err)
//
//	signerIDs, err := signature.DecodeSignerIndicesToIdentifiers(committee, signerIndices)
//	require.NoError(t, err)
//
//	data, err := packer.Decode(sig)
//	require.NoError(t, err)
//
//	data.SigType = []byte{1}
//
//	encoded, err := packer.Encode(data)
//	require.NoError(t, err)
//
//	_, err = packer.Unpack(signerIDs, encoded)
//	require.True(t, model.IsInvalidFormatError(err))
//}
//
//// TestPackUnpackWithoutRBAggregatedSig test that a packed data without random beacon signers and
//// aggregated random beacon sig can be correctly packed and unpacked
//// given the consensus committee [A, B, C]
//// [A, B, C] are non-random beacon nodes
//// aggregated staking sigs are from [A,B,C]
//// no aggregated random beacon sigs
//// no random beacon signers
//func TestPackUnpackWithoutRBAggregatedSig(t *testing.T) {
//	identities := unittest.IdentityListFixture(3, unittest.WithRole(flow.RoleConsensus))
//	committee := identities.NodeIDs()
//
//	// prepare data for testing
//	blockID := unittest.IdentifierFixture()
//
//	blockSigData := &hotstuff.BlockSignatureData{
//		StakingSigners:               committee,
//		RandomBeaconSigners:          nil,
//		AggregatedStakingSig:         unittest.SignatureFixture(),
//		AggregatedRandomBeaconSig:    nil,
//		ReconstructedRandomBeaconSig: unittest.SignatureFixture(),
//	}
//
//	// create packer with the committee
//	packer := newPacker(identities)
//
//	// pack & unpack
//	signerIndices, sig, err := packer.Pack(blockID, blockSigData)
//	require.NoError(t, err)
//
//	signerIDs, err := signature.DecodeSignerIndicesToIdentifiers(committee, signerIndices)
//	require.NoError(t, err)
//
//	unpacked, err := packer.Unpack(signerIDs, sig)
//	require.NoError(t, err)
//
//	// check that the unpack data match with the original data
//	require.Equal(t, blockSigData.StakingSigners, unpacked.StakingSigners)
//	require.Equal(t, blockSigData.AggregatedStakingSig, unpacked.AggregatedStakingSig)
//	require.Equal(t, blockSigData.ReconstructedRandomBeaconSig, unpacked.ReconstructedRandomBeaconSig)
//
//	// we need to specifically test if it's empty, it has to be by test definition
//	require.Empty(t, unpacked.RandomBeaconSigners)
//	require.Empty(t, unpacked.AggregatedRandomBeaconSig)
//
//	// check the packed signer IDs
//	expectedSignerIDs := append([]flow.Identifier{}, blockSigData.StakingSigners...)
//	require.Equal(t, expectedSignerIDs, signerIDs)
//}
//
//// TestPackWithoutRBAggregatedSig tests that packer correctly handles BlockSignatureData
//// with different structure format, more specifically there is no difference between
//// nil and empty slices for RandomBeaconSigners and AggregatedRandomBeaconSig.
//func TestPackWithoutRBAggregatedSig(t *testing.T) {
//	identities := unittest.IdentityListFixture(3, unittest.WithRole(flow.RoleConsensus))
//	committee := identities.NodeIDs()
//
//	// prepare data for testing
//	blockID := unittest.IdentifierFixture()
//
//	aggregatedSig := unittest.SignatureFixture()
//	reconstructedSig := unittest.SignatureFixture()
//
//	blockSigDataWithEmptySlices := &hotstuff.BlockSignatureData{
//		StakingSigners:               committee,
//		RandomBeaconSigners:          []flow.Identifier{},
//		AggregatedStakingSig:         aggregatedSig,
//		AggregatedRandomBeaconSig:    []byte{},
//		ReconstructedRandomBeaconSig: reconstructedSig,
//	}
//
//	blockSigDataWithNils := &hotstuff.BlockSignatureData{
//		StakingSigners:               committee,
//		RandomBeaconSigners:          nil,
//		AggregatedStakingSig:         aggregatedSig,
//		AggregatedRandomBeaconSig:    nil,
//		ReconstructedRandomBeaconSig: reconstructedSig,
//	}
//
//	// create packer with the committee
//	packer := newPacker(identities)
//
//	// pack
//	signerIDs_A, sig_A, err := packer.Pack(blockID, blockSigDataWithEmptySlices)
//	require.NoError(t, err)
//
//	signerIDs_B, sig_B, err := packer.Pack(blockID, blockSigDataWithNils)
//	require.NoError(t, err)
//
//	// should be the same
//	require.Equal(t, signerIDs_A, signerIDs_B)
//	require.Equal(t, sig_A, sig_B)
//}

//
//func TestSerializeAndDeserializeSigTypes(t *testing.T) {
//	t.Run("nothing", func(t *testing.T) {
//		expected := []encoding.SigType{}
//		bytes, err := serializeToBitVector(expected)
//		require.NoError(t, err)
//		require.Equal(t, []byte{}, bytes)
//
//		types, err := deserializeFromBitVector(bytes, len(expected))
//		require.NoError(t, err)
//		require.Equal(t, expected, types)
//	})
//
//	t.Run("1 SigTypeStaking", func(t *testing.T) {
//		expected := []encoding.SigType{encoding.SigTypeStaking}
//		bytes, err := serializeToBitVector(expected)
//		require.NoError(t, err)
//		require.Equal(t, []byte{0}, bytes)
//
//		types, err := deserializeFromBitVector(bytes, len(expected))
//		require.NoError(t, err)
//		require.Equal(t, expected, types)
//	})
//
//	t.Run("1 SigTypeRandomBeacon", func(t *testing.T) {
//		expected := []encoding.SigType{encoding.SigTypeRandomBeacon}
//		bytes, err := serializeToBitVector(expected)
//		require.NoError(t, err)
//		require.Equal(t, []byte{1 << 7}, bytes)
//
//		types, err := deserializeFromBitVector(bytes, len(expected))
//		require.NoError(t, err)
//		require.Equal(t, expected, types)
//	})
//
//	t.Run("2 SigTypeRandomBeacon", func(t *testing.T) {
//		expected := []encoding.SigType{
//			encoding.SigTypeRandomBeacon,
//			encoding.SigTypeRandomBeacon,
//		}
//		bytes, err := serializeToBitVector(expected)
//		require.NoError(t, err)
//		require.Equal(t, []byte{1<<7 + 1<<6}, bytes)
//
//		types, err := deserializeFromBitVector(bytes, len(expected))
//		require.NoError(t, err)
//		require.Equal(t, expected, types)
//	})
//
//	t.Run("8 SigTypeRandomBeacon", func(t *testing.T) {
//		count := 8
//		expected := make([]encoding.SigType, 0)
//		for i := 0; i < count; i++ {
//			expected = append(expected, encoding.SigTypeRandomBeacon)
//		}
//		bytes, err := serializeToBitVector(expected)
//		require.NoError(t, err)
//		require.Equal(t, []byte{255}, bytes)
//
//		types, err := deserializeFromBitVector(bytes, len(expected))
//		require.NoError(t, err)
//		require.Equal(t, expected, types)
//	})
//
//	t.Run("8 SigTypeStaking", func(t *testing.T) {
//		count := 8
//		expected := make([]encoding.SigType, 0)
//		for i := 0; i < count; i++ {
//			expected = append(expected, encoding.SigTypeStaking)
//		}
//		bytes, err := serializeToBitVector(expected)
//		require.NoError(t, err)
//		require.Equal(t, []byte{0}, bytes)
//
//		types, err := deserializeFromBitVector(bytes, len(expected))
//		require.NoError(t, err)
//		require.Equal(t, expected, types)
//	})
//
//	t.Run("9 SigTypeRandomBeacon", func(t *testing.T) {
//		count := 9
//		expected := make([]encoding.SigType, 0)
//		for i := 0; i < count; i++ {
//			expected = append(expected, encoding.SigTypeRandomBeacon)
//		}
//		bytes, err := serializeToBitVector(expected)
//		require.NoError(t, err)
//		require.Equal(t, []byte{255, 1 << 7}, bytes)
//
//		types, err := deserializeFromBitVector(bytes, len(expected))
//		require.NoError(t, err)
//		require.Equal(t, expected, types)
//	})
//
//	t.Run("9 SigTypeStaking", func(t *testing.T) {
//		count := 9
//		expected := make([]encoding.SigType, 0)
//		for i := 0; i < count; i++ {
//			expected = append(expected, encoding.SigTypeStaking)
//		}
//		bytes, err := serializeToBitVector(expected)
//		require.NoError(t, err)
//		require.Equal(t, []byte{0, 0}, bytes)
//
//		types, err := deserializeFromBitVector(bytes, len(expected))
//		require.NoError(t, err)
//		require.Equal(t, expected, types)
//	})
//
//	t.Run("16 SigTypeRandomBeacon, 2 groups", func(t *testing.T) {
//		count := 16
//		expected := make([]encoding.SigType, 0)
//		for i := 0; i < count; i++ {
//			expected = append(expected, encoding.SigTypeRandomBeacon)
//		}
//		bytes, err := serializeToBitVector(expected)
//		require.NoError(t, err)
//		require.Equal(t, []byte{255, 255}, bytes)
//
//		types, err := deserializeFromBitVector(bytes, len(expected))
//		require.NoError(t, err)
//		require.Equal(t, expected, types)
//	})
//
//	t.Run("3 SigTypeRandomBeacon, 4 SigTypeStaking", func(t *testing.T) {
//		random, staking := 3, 4
//		expected := make([]encoding.SigType, 0)
//		for i := 0; i < random; i++ {
//			expected = append(expected, encoding.SigTypeRandomBeacon)
//		}
//		for i := 0; i < staking; i++ {
//			expected = append(expected, encoding.SigTypeStaking)
//		}
//		bytes, err := serializeToBitVector(expected)
//		require.NoError(t, err)
//		require.Equal(t, []byte{1<<7 + 1<<6 + 1<<5}, bytes)
//
//		types, err := deserializeFromBitVector(bytes, len(expected))
//		require.NoError(t, err)
//		require.Equal(t, expected, types)
//	})
//
//	t.Run("3 SigTypeStaking, 4 SigTypeRandomBeacon", func(t *testing.T) {
//		staking, random := 3, 4
//		expected := make([]encoding.SigType, 0)
//		for i := 0; i < staking; i++ {
//			expected = append(expected, encoding.SigTypeStaking)
//		}
//		for i := 0; i < random; i++ {
//			expected = append(expected, encoding.SigTypeRandomBeacon)
//		}
//		bytes, err := serializeToBitVector(expected)
//		require.NoError(t, err)
//		// 00011110
//		require.Equal(t, []byte{1<<4 + 1<<3 + 1<<2 + 1<<1}, bytes)
//
//		types, err := deserializeFromBitVector(bytes, len(expected))
//		require.NoError(t, err)
//		require.Equal(t, expected, types)
//	})
//
//	t.Run("3 SigTypeStaking, 6 SigTypeRandomBeacon", func(t *testing.T) {
//		staking, random := 3, 6
//		expected := make([]encoding.SigType, 0)
//		for i := 0; i < staking; i++ {
//			expected = append(expected, encoding.SigTypeStaking)
//		}
//		for i := 0; i < random; i++ {
//			expected = append(expected, encoding.SigTypeRandomBeacon)
//		}
//		bytes, err := serializeToBitVector(expected)
//		require.NoError(t, err)
//		// 00011110, 10000000
//		require.Equal(t, []byte{1<<4 + 1<<3 + 1<<2 + 1<<1 + 1, 1 << 7}, bytes)
//
//		types, err := deserializeFromBitVector(bytes, len(expected))
//		require.NoError(t, err)
//		require.Equal(t, expected, types)
//	})
//}
//
//func TestDeserializeMismatchingBytes(t *testing.T) {
//	count := 9
//	expected := make([]encoding.SigType, 0)
//	for i := 0; i < count; i++ {
//		expected = append(expected, encoding.SigTypeStaking)
//	}
//	bytes, err := serializeToBitVector(expected)
//	require.NoError(t, err)
//
//	for invalidCount := 0; invalidCount < 100; invalidCount++ {
//		if invalidCount >= count && invalidCount <= 16 {
//			// skip correct count
//			continue
//		}
//		_, err := deserializeFromBitVector(bytes, invalidCount)
//		require.Error(t, err, fmt.Sprintf("invalid count: %v", invalidCount))
//		require.True(t, model.IsInvalidFormatError(err), fmt.Sprintf("invalid count: %v", invalidCount))
//	}
//}
//
//func TestDeserializeInvalidTailingBits(t *testing.T) {
//	_, err := deserializeFromBitVector([]byte{255, 1<<7 + 1<<1}, 9)
//	require.True(t, model.IsInvalidFormatError(err))
//	require.Contains(t, fmt.Sprintf("%v", err), "remaining bits")
//}

// sampleSigners takes `committee` and samples to _disjoint_ subsets
// (`stakingSigners` and `randomBeaconSigners`) with the specified cardinality
func sampleSigners(
	committee flow.IdentifierList,
	numStakingSigners int,
	numRandomBeaconSigners int,
) (stakingSigners flow.IdentifierList, randomBeaconSigners flow.IdentifierList) {
	if numStakingSigners+numRandomBeaconSigners > len(committee) {
		panic(fmt.Sprintf("Cannot sample %d nodes out of a committee is size %d", numStakingSigners+numRandomBeaconSigners, len(committee)))
	}

	stakingSigners = committee.Sample(uint(numStakingSigners))
	remaining := committee.Filter(id.Not(id.In(stakingSigners...)))
	randomBeaconSigners = remaining.Sample(uint(numRandomBeaconSigners))
	return
}

// correctEncoding verifies that the given indices conform to the following specification:
//  * indices is the _smallest_ possible byte slice that contains at least `len(canonicalIdentifiers)` number of _bits_
//  * Let indices[i] denote the ith bit of `indices`. We verify that:
//                       ┌ 1 if and only if canonicalIdentifiers[i] is in `subset`
//          indices[i] = └ 0 otherwise
// This function can be used to verify signer indices as well as signature type encoding
func correctEncoding(t require.TestingT, indices []byte, canonicalIdentifiers flow.IdentifierList, subset flow.IdentifierList) {
	// verify that indices has correct length
	numberBits := 8 * len(indices)
	require.True(t, numberBits >= len(canonicalIdentifiers), "signerIndices has too few bits")
	require.True(t, numberBits-len(canonicalIdentifiers) < 8, "signerIndices is padded with too many bits")

	// convert canonicalIdentifiers to map Identifier -> index
	m := make(map[flow.Identifier]int)
	for i, id := range canonicalIdentifiers {
		m[id] = i
	}

	// make sure that every member of the subset is represented by a 1 in `indices`
	for _, id := range subset {
		bitIndex := m[id]
		require.True(t, bitutils.ReadBit(indices, bitIndex) == 1)
		delete(m, id)
	}

	// as we delete all IDs in subset from m, the remaining ID in `m` should be represented by a 0 in `indices`
	for id := range m {
		bitIndex := m[id]
		require.True(t, bitutils.ReadBit(indices, bitIndex) == 0)
	}

	// the padded bits should also all be 0:
	for i := len(canonicalIdentifiers); i < 8*len(indices); i++ {
		require.True(t, bitutils.ReadBit(indices, i) == 0)
	}
}
