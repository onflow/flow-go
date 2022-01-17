package merkle

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/blake2b"
)

var payload1, _ = hex.DecodeString("62b0326507ebce9d4a242908d20559ceca965c5e9848646bd0c05047c8487aadfcb3d851e77e5a055d306e48c376f8")
var payload2, _ = hex.DecodeString("bab02e6213dfad3546aa473922bba0")

// TestBlakeMAC verifies that constructor blake2b.New256(m) never errors for any of the used MAC
// values `m`. This is assumed by the Node's Hash() implementations. We test this assumption holds
// at build time, but avoid runtime-checks for performance reasons.
// Context: The blake2b MAC key should be less than 64 bytes. Constructor blake2b.New256(m) checks
// this condition and errors, but we ignore its error return at runtime.
func TestBlakeMAC(t *testing.T) {
	var e error

	// leaf nodes
	assert.True(t, len(leafNodeTag) < 64)
	_, e = blake2b.New256(leafNodeTag[:])
	assert.NoError(t, e)

	// full nodes
	assert.True(t, len(fullNodeTag) < 64)
	_, e = blake2b.New256(fullNodeTag[:])
	assert.NoError(t, e)

	// short nodes
	assert.True(t, len(shortNodeTag) < 64)
	_, e = blake2b.New256(shortNodeTag[:])
	assert.NoError(t, e)

}

// TestLeafHash verifies that the hash of a leaf returns the expected value.
// We compare with a python-reference implementation
func TestLeafHash(t *testing.T) {
	// reference value (from python reference implementation)
	ref := "1b30482d4dc8c1a8d846d05765c03a33f0267b56b9a7be8defe38958f89c95fc"

	l := leaf{val: payload1}
	require.Equal(t, ref, hex.EncodeToString(l.Hash()))
}

// TestShortHash verifies that the hash of a short node returns the expected value.
// We compare with a python-reference implementation.
func TestShortHash(t *testing.T) {
	t.Run("13-bit path", func(t *testing.T) {
		// expected value from python reference implementation
		ref := "a5632447428968cca925e802e5251c2c2b31e4ebf5236a3a66a60fe0509d6e40"

		// path of 13 bits: 1011001010011
		// per convention, we pad zeros to the end to obtain full bytes:
		//  -> 10110010 10011000 (padded binary representation)
		//  ->      178      152 (uint8 representation)
		//
		path := []byte{178, 152}
		s := short{
			path:  path,
			count: 13,
			child: &leaf{val: payload1},
		}
		require.Equal(t, ref, hex.EncodeToString(s.Hash()))
	})

	t.Run("65536-bit path", func(t *testing.T) {
		// as path, we just repeat the following 32 bytes 256 times
		k, _ := hex.DecodeString("1b30482d4dc8c1a8d846d05765c03a33f0267b56b9a7be8defe38958f89c95fc")
		path := make([]byte, 0, 8192)
		for i := 1; i <= 256; i++ {
			path = append(path, k...)
		}
		// expected value from python reference implementation
		ref := "b49541ce56e5cd207cac3a08821ddfc61fc5aaf8886907d226581a7c53a48827"

		s := short{
			path:  path,
			count: 65536,
			child: &leaf{val: payload1},
		}
		require.Equal(t, ref, hex.EncodeToString(s.Hash()))
	})
}

// Test_ShortNodePathLengthEncoding:
// The tree enforces a max key length of `maxKeyLength`. We verify that:
// 1. the resulting number of bits (i.e. maxKeyLength * 8), does not
//    overflow the hardware-dependent int range.
// 2. the value range from [1, ..., maxKeyLength * 8] can be encoded into 2 bytes,
//    as this is required by the short node (but not enforced at run time)
// 3. serializedPathSegmentLength(l) implements the following encoding
//    * if 1 ≤ l ≤ 65535: we represent l as unsigned int with big-endian encoding
//    * for l = 65536: we represent l as binary 00000000 00000000
// Comment: enforcing that we never exceed the limit 65536 is implemented on the trie-level
func Test_ShortNodePathLengthEncoding(t *testing.T) {
	// testing 1:
	maxInt := int(^uint(0) >> 1) // largest int value (hardware-dependent)
	require.True(t, maxKeyLength <= maxInt/8)

	// testing 2:
	// two bytes can encode 2^16 = 65536 different values
	require.Equal(t, uint64(65536), uint64(maxKeyLength)*8)

	// testing 3:
	require.Equal(t, [2]byte{0, 1}, serializedPathSegmentLength(1))
	require.Equal(t, [2]byte{255, 255}, serializedPathSegmentLength(65535))
	require.Equal(t, [2]byte{0, 0}, serializedPathSegmentLength(65536))
}

// TestFullHash verifies that the hash of a full node returns the expected value.
// We compare with a python-reference implementation.
func TestFullHash(t *testing.T) {
	// reference value (from python reference implementation)
	ref := "6edee16badebe695a2ff7df90e429ba66e8986f7c9d089e4ad8fccbd89b0ccc8"

	f := full{
		left:  &leaf{val: payload1},
		right: &leaf{val: payload2},
	}
	require.Equal(t, ref, hex.EncodeToString(f.Hash()))
}
