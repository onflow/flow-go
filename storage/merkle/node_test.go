package merkle

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/blake2b"
)

var payload1, _ = hex.DecodeString("62b0326507ebce9d4a242908d20559ceca965c5e9848646bd0c05047c8487aadfcb3d851e77e5a055d306e48c376f8")
var payload2, _ = hex.DecodeString("bab02e6213dfad3546aa473922bba0")

// TestBlakeMac verifies that constructor blake2b.New256(m) never errors for any of the used MAC
// values `m`. This is assumed by the Node's Hash() implementations. We test this assumption holds
// at build time, but avoid runtime-checks for performance reasons.
// Context: The blake2b MAC key should be less than 64 bytes. Constructor blake2b.New256(m) checks
// this condition and errors, but we ignore its error return at runtime.
func TestBlakeMac(t *testing.T) {
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
	// reference value (from python reference implementation)
	ref := "ae45d4f19809443f57d15bab02e6213dfad3546aa473922bba0eebda4e51b5ae"

	// path of 13 bits: 1011001010011
	// per convention, we pad zeros to the end to obtain full bytes:
	//  -> 10110010 10011000 (padded binary representation)
	//  ->      178      152 (uint8 representation)
	//
	path := []byte{178, 152}
	fmt.Println(hex.EncodeToString(path))
	s := short{
		path:  path,
		count: 13,
		child: &leaf{val: payload1},
	}
	require.Equal(t, ref, hex.EncodeToString(s.Hash()))
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
