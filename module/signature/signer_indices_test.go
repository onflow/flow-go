package signature_test

import (
	"fmt"
	"testing"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/encoding"

	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
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

func TestEncodeIdentity(t *testing.T) {
	only := unittest.IdentifierListFixture(1)
	indices, err := signature.EncodeSignerIdentifiersToIndices(only, only)
	require.NoError(t, err)
	// byte(1,0,0,0,0,0,0,0)
	require.Equal(t, []byte{byte(1 << 7)}, indices)
}

// TestEncodeFail verifies that an error is returned in case some signer is not part
// of the set of canonicalIdentifiers
func TestEncodeFail(t *testing.T) {
	fullIdentities := unittest.IdentifierListFixture(20)
	_, err := signature.EncodeSignersToIndices(fullIdentities[1:], fullIdentities[:10])
	require.Error(t, err)
}

func TestSerializeAndDeserializeSigTypes(t *testing.T) {
	t.Run("nothing", func(t *testing.T) {
		expected := []encoding.SigType{}
		bytes, err := serializeToBitVector(expected)
		require.NoError(t, err)
		require.Equal(t, []byte{}, bytes)

		types, err := deserializeFromBitVector(bytes, len(expected))
		require.NoError(t, err)
		require.Equal(t, expected, types)
	})

	t.Run("1 SigTypeStaking", func(t *testing.T) {
		expected := []encoding.SigType{encoding.SigTypeStaking}
		bytes, err := serializeToBitVector(expected)
		require.NoError(t, err)
		require.Equal(t, []byte{0}, bytes)

		types, err := deserializeFromBitVector(bytes, len(expected))
		require.NoError(t, err)
		require.Equal(t, expected, types)
	})

	t.Run("1 SigTypeRandomBeacon", func(t *testing.T) {
		expected := []encoding.SigType{encoding.SigTypeRandomBeacon}
		bytes, err := serializeToBitVector(expected)
		require.NoError(t, err)
		require.Equal(t, []byte{1 << 7}, bytes)

		types, err := deserializeFromBitVector(bytes, len(expected))
		require.NoError(t, err)
		require.Equal(t, expected, types)
	})

	t.Run("2 SigTypeRandomBeacon", func(t *testing.T) {
		expected := []encoding.SigType{
			encoding.SigTypeRandomBeacon,
			encoding.SigTypeRandomBeacon,
		}
		bytes, err := serializeToBitVector(expected)
		require.NoError(t, err)
		require.Equal(t, []byte{1<<7 + 1<<6}, bytes)

		types, err := deserializeFromBitVector(bytes, len(expected))
		require.NoError(t, err)
		require.Equal(t, expected, types)
	})

	t.Run("8 SigTypeRandomBeacon", func(t *testing.T) {
		count := 8
		expected := make([]encoding.SigType, 0)
		for i := 0; i < count; i++ {
			expected = append(expected, encoding.SigTypeRandomBeacon)
		}
		bytes, err := serializeToBitVector(expected)
		require.NoError(t, err)
		require.Equal(t, []byte{255}, bytes)

		types, err := deserializeFromBitVector(bytes, len(expected))
		require.NoError(t, err)
		require.Equal(t, expected, types)
	})

	t.Run("8 SigTypeStaking", func(t *testing.T) {
		count := 8
		expected := make([]encoding.SigType, 0)
		for i := 0; i < count; i++ {
			expected = append(expected, encoding.SigTypeStaking)
		}
		bytes, err := serializeToBitVector(expected)
		require.NoError(t, err)
		require.Equal(t, []byte{0}, bytes)

		types, err := deserializeFromBitVector(bytes, len(expected))
		require.NoError(t, err)
		require.Equal(t, expected, types)
	})

	t.Run("9 SigTypeRandomBeacon", func(t *testing.T) {
		count := 9
		expected := make([]encoding.SigType, 0)
		for i := 0; i < count; i++ {
			expected = append(expected, encoding.SigTypeRandomBeacon)
		}
		bytes, err := serializeToBitVector(expected)
		require.NoError(t, err)
		require.Equal(t, []byte{255, 1 << 7}, bytes)

		types, err := deserializeFromBitVector(bytes, len(expected))
		require.NoError(t, err)
		require.Equal(t, expected, types)
	})

	t.Run("9 SigTypeStaking", func(t *testing.T) {
		count := 9
		expected := make([]encoding.SigType, 0)
		for i := 0; i < count; i++ {
			expected = append(expected, encoding.SigTypeStaking)
		}
		bytes, err := serializeToBitVector(expected)
		require.NoError(t, err)
		require.Equal(t, []byte{0, 0}, bytes)

		types, err := deserializeFromBitVector(bytes, len(expected))
		require.NoError(t, err)
		require.Equal(t, expected, types)
	})

	t.Run("16 SigTypeRandomBeacon, 2 groups", func(t *testing.T) {
		count := 16
		expected := make([]encoding.SigType, 0)
		for i := 0; i < count; i++ {
			expected = append(expected, encoding.SigTypeRandomBeacon)
		}
		bytes, err := serializeToBitVector(expected)
		require.NoError(t, err)
		require.Equal(t, []byte{255, 255}, bytes)

		types, err := deserializeFromBitVector(bytes, len(expected))
		require.NoError(t, err)
		require.Equal(t, expected, types)
	})

	t.Run("3 SigTypeRandomBeacon, 4 SigTypeStaking", func(t *testing.T) {
		random, staking := 3, 4
		expected := make([]encoding.SigType, 0)
		for i := 0; i < random; i++ {
			expected = append(expected, encoding.SigTypeRandomBeacon)
		}
		for i := 0; i < staking; i++ {
			expected = append(expected, encoding.SigTypeStaking)
		}
		bytes, err := serializeToBitVector(expected)
		require.NoError(t, err)
		require.Equal(t, []byte{1<<7 + 1<<6 + 1<<5}, bytes)

		types, err := deserializeFromBitVector(bytes, len(expected))
		require.NoError(t, err)
		require.Equal(t, expected, types)
	})

	t.Run("3 SigTypeStaking, 4 SigTypeRandomBeacon", func(t *testing.T) {
		staking, random := 3, 4
		expected := make([]encoding.SigType, 0)
		for i := 0; i < staking; i++ {
			expected = append(expected, encoding.SigTypeStaking)
		}
		for i := 0; i < random; i++ {
			expected = append(expected, encoding.SigTypeRandomBeacon)
		}
		bytes, err := serializeToBitVector(expected)
		require.NoError(t, err)
		// 00011110
		require.Equal(t, []byte{1<<4 + 1<<3 + 1<<2 + 1<<1}, bytes)

		types, err := deserializeFromBitVector(bytes, len(expected))
		require.NoError(t, err)
		require.Equal(t, expected, types)
	})

	t.Run("3 SigTypeStaking, 6 SigTypeRandomBeacon", func(t *testing.T) {
		staking, random := 3, 6
		expected := make([]encoding.SigType, 0)
		for i := 0; i < staking; i++ {
			expected = append(expected, encoding.SigTypeStaking)
		}
		for i := 0; i < random; i++ {
			expected = append(expected, encoding.SigTypeRandomBeacon)
		}
		bytes, err := serializeToBitVector(expected)
		require.NoError(t, err)
		// 00011110, 10000000
		require.Equal(t, []byte{1<<4 + 1<<3 + 1<<2 + 1<<1 + 1, 1 << 7}, bytes)

		types, err := deserializeFromBitVector(bytes, len(expected))
		require.NoError(t, err)
		require.Equal(t, expected, types)
	})
}

func TestDeserializeMismatchingBytes(t *testing.T) {
	count := 9
	expected := make([]encoding.SigType, 0)
	for i := 0; i < count; i++ {
		expected = append(expected, encoding.SigTypeStaking)
	}
	bytes, err := serializeToBitVector(expected)
	require.NoError(t, err)

	for invalidCount := 0; invalidCount < 100; invalidCount++ {
		if invalidCount >= count && invalidCount <= 16 {
			// skip correct count
			continue
		}
		_, err := deserializeFromBitVector(bytes, invalidCount)
		require.Error(t, err, fmt.Sprintf("invalid count: %v", invalidCount))
		require.True(t, model.IsInvalidFormatError(err), fmt.Sprintf("invalid count: %v", invalidCount))
	}
}

func TestDeserializeInvalidTailingBits(t *testing.T) {
	_, err := deserializeFromBitVector([]byte{255, 1<<7 + 1<<1}, 9)
	require.True(t, model.IsInvalidFormatError(err))
	require.Contains(t, fmt.Sprintf("%v", err), "remaining bits")
}
