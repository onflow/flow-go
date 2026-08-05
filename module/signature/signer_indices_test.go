package signature_test

import (
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/onflow/flow-go/ledger/common/bitutils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/flow/filter/id"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestEncodeDecodeIdentities verifies the two path of encoding -> decoding:
//  1. Identifiers --encode--> Indices --decode--> Identifiers
//  2. for the decoding step, we offer an optimized convenience function to directly
//     decode to full identities: Indices --decode--> Identities
func TestEncodeDecodeIdentities(t *testing.T) {
	canonicalIdentities := unittest.IdentityListFixture(20).Sort(flow.Canonical[flow.Identity]).ToSkeleton()
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

			// decoding option 2: decode to Identities
			decodedIdentities, err := signature.DecodeSignerIndicesToIdentities(canonicalIdentities, indices)
			require.NoError(t, err)
			require.Equal(t, signers, decodedIdentities)
		}
	}
}

func TestEncodeDecodeIdentitiesFail(t *testing.T) {
	canonicalIdentities := unittest.IdentityListFixture(20)
	canonicalIdentifiers := canonicalIdentities.NodeIDs()
	signers := canonicalIdentities[3:19]
	validIndices, err := signature.EncodeSignersToIndices(canonicalIdentities.NodeIDs(), signers.NodeIDs())
	require.NoError(t, err)

	_, err = signature.DecodeSignerIndicesToIdentifiers(canonicalIdentifiers, validIndices)
	require.NoError(t, err)

	invalidSum := make([]byte, len(validIndices))
	copy(invalidSum, validIndices)
	if invalidSum[0] == byte(0) {
		invalidSum[0] = byte(1)
	} else {
		invalidSum[0] = byte(0)
	}
	_, err = signature.DecodeSignerIndicesToIdentifiers(canonicalIdentifiers, invalidSum)
	require.True(t, signature.IsInvalidSignerIndicesError(err), err)
	require.ErrorIs(t, err, signature.ErrInvalidChecksum, err)

	incompatibleLength := append(validIndices, byte(0))
	_, err = signature.DecodeSignerIndicesToIdentifiers(canonicalIdentifiers, incompatibleLength)
	require.True(t, signature.IsInvalidSignerIndicesError(err), err)
	require.False(t, signature.IsInvalidSignerIndicesError(signature.NewInvalidSigTypesErrorf("sdf")))
	require.ErrorIs(t, err, signature.ErrIncompatibleBitVectorLength, err)

	illegallyPadded := make([]byte, len(validIndices))
	copy(illegallyPadded, validIndices)
	illegallyPadded[len(illegallyPadded)-1]++
	_, err = signature.DecodeSignerIndicesToIdentifiers(canonicalIdentifiers, illegallyPadded)
	require.True(t, signature.IsInvalidSignerIndicesError(err), err)
	require.ErrorIs(t, err, signature.ErrIllegallyPaddedBitVector, err)
}

func TestEncodeIdentity(t *testing.T) {
	only := unittest.IdentifierListFixture(1)
	indices, err := signature.EncodeSignersToIndices(only, only)
	require.NoError(t, err)
	// byte(1,0,0,0,0,0,0,0)
	require.Equal(t, []byte{byte(1 << 7)}, indices[signature.CheckSumLen:])
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
		committeeSize := rapid.IntRange(1, 272).Draw(t, "committeeSize")
		numStakingSigners := rapid.IntRange(0, committeeSize).Draw(t, "numStakingSigners")
		numRandomBeaconSigners := rapid.IntRange(0, committeeSize-numStakingSigners).Draw(t, "numRandomBeaconSigners")

		// create committee
		committeeIdentities := unittest.IdentityListFixture(committeeSize, unittest.WithRole(flow.RoleConsensus)).Sort(flow.Canonical[flow.Identity])
		committee := committeeIdentities.NodeIDs()
		stakingSigners, beaconSigners := sampleSigners(t, committee, numStakingSigners, numRandomBeaconSigners)

		// encode
		prefixed, sigTypes, err := signature.EncodeSignerToIndicesAndSigType(committee, stakingSigners, beaconSigners)
		require.NoError(t, err)

		signerIndices, err := signature.CompareAndExtract(committeeIdentities.NodeIDs(), prefixed)
		require.NoError(t, err)

		// check verify signer indices
		unorderedSigners := stakingSigners.Union(beaconSigners) // caution, the Union operation potentially changes the ordering
		correctEncoding(t, signerIndices, committee, unorderedSigners)

		// check sigTypes
		canSigners := committeeIdentities.Filter(filter.HasNodeID[flow.Identity](unorderedSigners...)).NodeIDs() // generates list of signer IDs in canonical order
		correctEncoding(t, sigTypes, canSigners, beaconSigners)
	})
}

// Test_DecodeSigTypeToStakingAndBeaconSigners uses fuzzy-testing framework Rapid to
// test the method DecodeSigTypeToStakingAndBeaconSigners:
//   - we generate a set of authorized signer: `committeeIdentities`
//   - part of this set is sampled as staking singers: `stakingSigners`
//   - another part of `committeeIdentities` is sampled as beacon singers: `beaconSigners`
//   - we encode the set and check that the results conform to the protocol specification
//   - We encode the set using `EncodeSignerToIndicesAndSigType` (tested before) and then decode it.
//     Thereby we should recover the original input. Caution, the order might be different,
//     so we sort both sets.
func Test_DecodeSigTypeToStakingAndBeaconSigners(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// select total committee size, number of random beacon signers and number of staking signers
		committeeSize := rapid.IntRange(1, 272).Draw(t, "committeeSize")
		numStakingSigners := rapid.IntRange(0, committeeSize).Draw(t, "numStakingSigners")
		numRandomBeaconSigners := rapid.IntRange(0, committeeSize-numStakingSigners).Draw(t, "numRandomBeaconSigners")

		// create committee
		committeeIdentities := unittest.IdentityListFixture(committeeSize, unittest.WithRole(flow.RoleConsensus)).
			Sort(flow.Canonical[flow.Identity])
		committee := committeeIdentities.NodeIDs()
		stakingSigners, beaconSigners := sampleSigners(t, committee, numStakingSigners, numRandomBeaconSigners)

		// encode
		signerIndices, sigTypes, err := signature.EncodeSignerToIndicesAndSigType(committee, stakingSigners, beaconSigners)
		require.NoError(t, err)

		// decode
		decSignerIdentites, err := signature.DecodeSignerIndicesToIdentities(committeeIdentities.ToSkeleton(), signerIndices)
		require.NoError(t, err)
		decStakingSigners, decBeaconSigners, err := signature.DecodeSigTypeToStakingAndBeaconSigners(decSignerIdentites, sigTypes)
		require.NoError(t, err)

		// verify; note that there is a slightly different convention between Filter and the decoding logic:
		// Filter returns nil for an empty list, while the decoding logic returns an instance of an empty slice
		sigIdentities := committeeIdentities.Filter(
			filter.Or(filter.HasNodeID[flow.Identity](stakingSigners...), filter.HasNodeID[flow.Identity](beaconSigners...))).ToSkeleton() // signer identities in canonical order
		if len(stakingSigners)+len(decBeaconSigners) > 0 {
			require.Equal(t, sigIdentities, decSignerIdentites)
		}
		if len(stakingSigners) == 0 {
			require.Empty(t, decStakingSigners)
		} else {
			require.Equal(t, committeeIdentities.Filter(filter.HasNodeID[flow.Identity](stakingSigners...)).ToSkeleton(), decStakingSigners)
		}
		if len(decBeaconSigners) == 0 {
			require.Empty(t, decBeaconSigners)
		} else {
			require.Equal(t, committeeIdentities.Filter(filter.HasNodeID[flow.Identity](beaconSigners...)).ToSkeleton(), decBeaconSigners)
		}
	})
}

func Test_ValidPaddingErrIncompatibleBitVectorLength(t *testing.T) {
	var err error
	// if bits is multiply of 8, then there is no padding needed, any sig type can be decoded.
	signers := unittest.IdentityListFixture(16).ToSkeleton()

	// 16 bits needs 2 bytes, provided 2 bytes
	_, _, err = signature.DecodeSigTypeToStakingAndBeaconSigners(signers, unittest.RandomBytes(2))
	require.NoError(t, err)

	// 1 byte less
	_, _, err = signature.DecodeSigTypeToStakingAndBeaconSigners(signers, []byte{byte(255)})
	require.True(t, signature.IsInvalidSigTypesError(err), "API-level error should be InvalidSigTypesError")
	require.ErrorIs(t, err, signature.ErrIncompatibleBitVectorLength, "low-level error representing the failure should be ErrIncompatibleBitVectorLength")

	// 1 byte more
	_, _, err = signature.DecodeSigTypeToStakingAndBeaconSigners(signers, []byte{})
	require.True(t, signature.IsInvalidSigTypesError(err), "API-level error should be InvalidSigTypesError")
	require.ErrorIs(t, err, signature.ErrIncompatibleBitVectorLength, "low-level error representing the failure should be ErrIncompatibleBitVectorLength")

	// if bits is not multiply of 8, then padding is needed
	signers = unittest.IdentityListFixture(15).ToSkeleton()
	_, _, err = signature.DecodeSigTypeToStakingAndBeaconSigners(signers, []byte{byte(255), byte(254)})
	require.NoError(t, err)

	// 1 byte more
	_, _, err = signature.DecodeSigTypeToStakingAndBeaconSigners(signers, []byte{byte(255), byte(255), byte(254)})
	require.True(t, signature.IsInvalidSigTypesError(err), "API-level error should be InvalidSigTypesError")
	require.ErrorIs(t, err, signature.ErrIncompatibleBitVectorLength, "low-level error representing the failure should be ErrIncompatibleBitVectorLength")

	// 1 byte less
	_, _, err = signature.DecodeSigTypeToStakingAndBeaconSigners(signers, []byte{byte(254)})
	require.True(t, signature.IsInvalidSigTypesError(err), "API-level error should be InvalidSigTypesError")
	require.ErrorIs(t, err, signature.ErrIncompatibleBitVectorLength, "low-level error representing the failure should be ErrIncompatibleBitVectorLength")

	// if bits is not multiply of 8,
	// 1 byte more
	signers = unittest.IdentityListFixture(0).ToSkeleton()
	_, _, err = signature.DecodeSigTypeToStakingAndBeaconSigners(signers, []byte{byte(255)})
	require.True(t, signature.IsInvalidSigTypesError(err), "API-level error should be InvalidSigTypesError")
	require.ErrorIs(t, err, signature.ErrIncompatibleBitVectorLength, "low-level error representing the failure should be ErrIncompatibleBitVectorLength")

	// 1 byte more
	signers = unittest.IdentityListFixture(1).ToSkeleton()
	_, _, err = signature.DecodeSigTypeToStakingAndBeaconSigners(signers, []byte{byte(0), byte(0)})
	require.True(t, signature.IsInvalidSigTypesError(err), "API-level error should be InvalidSigTypesError")
	require.ErrorIs(t, err, signature.ErrIncompatibleBitVectorLength, "low-level error representing the failure should be ErrIncompatibleBitVectorLength")

	// 1 byte less
	signers = unittest.IdentityListFixture(7).ToSkeleton()
	_, _, err = signature.DecodeSigTypeToStakingAndBeaconSigners(signers, []byte{})
	require.True(t, signature.IsInvalidSigTypesError(err), "API-level error should be InvalidSigTypesError")
	require.ErrorIs(t, err, signature.ErrIncompatibleBitVectorLength, "low-level error representing the failure should be ErrIncompatibleBitVectorLength")
}

func TestValidPaddingErrIllegallyPaddedBitVector(t *testing.T) {
	var signers flow.IdentitySkeletonList
	var err error
	// if bits is multiply of 8, then there is no padding needed, any sig type can be decoded.
	for count := 1; count < 8; count++ {
		signers = unittest.IdentityListFixture(count).ToSkeleton()
		_, _, err = signature.DecodeSigTypeToStakingAndBeaconSigners(signers, []byte{byte(255)}) // last bit should be 0, but 1
		require.True(t, signature.IsInvalidSigTypesError(err), "API-level error should be InvalidSigTypesError")
		require.ErrorIs(t, err, signature.ErrIllegallyPaddedBitVector, "low-level error representing the failure should be ErrIllegallyPaddedBitVector")

		_, _, err = signature.DecodeSigTypeToStakingAndBeaconSigners(signers, []byte{byte(1)}) // last bit should be 0, but 1
		require.True(t, signature.IsInvalidSigTypesError(err), "API-level error should be InvalidSigTypesError")
		require.ErrorIs(t, err, signature.ErrIllegallyPaddedBitVector, "low-level error representing the failure should be ErrIllegallyPaddedBitVector")
	}

	for count := 9; count < 16; count++ {
		signers = unittest.IdentityListFixture(count).ToSkeleton()
		_, _, err = signature.DecodeSigTypeToStakingAndBeaconSigners(signers, []byte{byte(255), byte(255)}) // last bit should be 0, but 1
		require.True(t, signature.IsInvalidSigTypesError(err), "API-level error should be InvalidSigTypesError")
		require.ErrorIs(t, err, signature.ErrIllegallyPaddedBitVector, "low-level error representing the failure should be ErrIllegallyPaddedBitVector")

		_, _, err = signature.DecodeSigTypeToStakingAndBeaconSigners(signers, []byte{byte(1), byte(1)}) // last bit should be 0, but 1
		require.True(t, signature.IsInvalidSigTypesError(err), "API-level error should be InvalidSigTypesError")
		require.ErrorIs(t, err, signature.ErrIllegallyPaddedBitVector, "low-level error representing the failure should be ErrIllegallyPaddedBitVector")
	}
}

// Test_EncodeSignersToIndices uses fuzzy-testing framework Rapid to test the method EncodeSignersToIndices:
// * we generate a set of authorized signer: `identities`
// * part of this set is sampled as singers: `signers`
// * we encode the set and check that the results conform to the protocol specification
func Test_EncodeSignersToIndices(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// select total committee size, number of random beacon signers and number of staking signers
		committeeSize := rapid.IntRange(1, 272).Draw(t, "committeeSize")
		numSigners := rapid.IntRange(0, committeeSize).Draw(t, "numSigners")

		// create committee
		identities := unittest.IdentityListFixture(committeeSize, unittest.WithRole(flow.RoleConsensus)).Sort(flow.Canonical[flow.Identity])
		committee := identities.NodeIDs()
		signers, err := committee.Sample(uint(numSigners))
		require.NoError(t, err)

		// encode
		prefixed, err := signature.EncodeSignersToIndices(committee, signers)
		require.NoError(t, err)

		signerIndices, err := signature.CompareAndExtract(committee, prefixed)
		require.NoError(t, err)

		// check verify signer indices
		correctEncoding(t, signerIndices, committee, signers)
	})
}

// Test_DecodeSignerIndicesToIdentifiers uses fuzzy-testing framework Rapid to test the method DecodeSignerIndicesToIdentifiers:
//   - we generate a set of authorized signer: `identities`
//   - part of this set is sampled as signers: `signers`
//   - We encode the set using `EncodeSignersToIndices` (tested before) and then decode it.
//     Thereby we should recover the original input. Caution, the order might be different,
//     so we sort both sets.
func Test_DecodeSignerIndicesToIdentifiers(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// select total committee size, number of random beacon signers and number of staking signers
		committeeSize := rapid.IntRange(1, 272).Draw(t, "committeeSize")
		numSigners := rapid.IntRange(0, committeeSize).Draw(t, "numSigners")

		// create committee
		identities := unittest.IdentityListFixture(committeeSize, unittest.WithRole(flow.RoleConsensus)).Sort(flow.Canonical[flow.Identity])
		committee := identities.NodeIDs()
		signers, err := committee.Sample(uint(numSigners))
		require.NoError(t, err)
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

const UpperBoundCommitteeSize = 272

func Test_DecodeSignerIndicesToIdentities(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// select total committee size, number of random beacon signers and number of staking signers
		committeeSize := rapid.IntRange(1, UpperBoundCommitteeSize).Draw(t, "committeeSize")
		numSigners := rapid.IntRange(0, committeeSize).Draw(t, "numSigners")

		// create committee
		identities := unittest.IdentityListFixture(committeeSize, unittest.WithRole(flow.RoleConsensus)).Sort(flow.Canonical[flow.Identity])
		fullSigners, err := identities.Sample(uint(numSigners))
		require.NoError(t, err)
		signers := fullSigners.ToSkeleton()

		// encode
		signerIndices, err := signature.EncodeSignersToIndices(identities.NodeIDs(), signers.NodeIDs())
		require.NoError(t, err)

		// decode and verify
		decodedSigners, err := signature.DecodeSignerIndicesToIdentities(identities.ToSkeleton(), signerIndices)
		require.NoError(t, err)

		require.Equal(t, signers.Sort(flow.Canonical[flow.IdentitySkeleton]), decodedSigners.Sort(flow.Canonical[flow.IdentitySkeleton]))
	})
}

// sampleSigners takes `committee` and samples to _disjoint_ subsets
// (`stakingSigners` and `randomBeaconSigners`) with the specified cardinality
func sampleSigners(
	t *rapid.T,
	committee flow.IdentifierList,
	numStakingSigners int,
	numRandomBeaconSigners int,
) (stakingSigners flow.IdentifierList, randomBeaconSigners flow.IdentifierList) {
	if numStakingSigners+numRandomBeaconSigners > len(committee) {
		panic(fmt.Sprintf("Cannot sample %d nodes out of a committee is size %d", numStakingSigners+numRandomBeaconSigners, len(committee)))
	}

	var err error
	stakingSigners, err = committee.Sample(uint(numStakingSigners))
	require.NoError(t, err)
	remaining := committee.Filter(id.Not(id.In(stakingSigners...)))
	randomBeaconSigners, err = remaining.Sample(uint(numRandomBeaconSigners))
	require.NoError(t, err)
	return
}

// correctEncoding verifies that the given indices conform to the following specification:
//   - indices is the _smallest_ possible byte slice that contains at least `len(canonicalIdentifiers)` number of _bits_
//   - Let indices[i] denote the ith bit of `indices`. We verify that:
//
// .                            ┌ 1 if and only if canonicalIdentifiers[i] is in `subset`
// .               indices[i] = └ 0 otherwise
//
// This function can be used to verify signer indices as well as signature type encoding
func correctEncoding(t require.TestingT, indices []byte, canonicalIdentifiers flow.IdentifierList, subset flow.IdentifierList) {
	// verify that indices has correct length
	numberBits := 8 * len(indices)
	require.True(t, numberBits >= len(canonicalIdentifiers), "signerIndices has too few bits")
	require.True(t, numberBits-len(canonicalIdentifiers) < 8, fmt.Sprintf("signerIndices %v is padded with too many %v bits",
		numberBits, len(canonicalIdentifiers)))

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
