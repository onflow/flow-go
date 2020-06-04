package trie_test

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/common"

	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/node"

	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/trie"
)

const (
	// ReferenceImplKeyByteSize is the key length in reference implementation: 2 bytes.
	// Please do NOT CHANGE.
	ReferenceImplKeyByteSize = 2
)

// TestEmptyTrie tests whether the root hash of an empty trie matches the formal specification.
// The expected value is coming from a reference implementation in python and hard-coded here.
func Test_EmptyTrie(t *testing.T) {
	// Make new Trie (independently of MForest):
	emptyTrie, err := trie.NewEmptyMTrie(ReferenceImplKeyByteSize, 23, []byte{})
	require.NoError(t, err)

	expectedRootHashHex := "6e24e2397f130d9d17bef32b19a77b8f5bcf03fb7e9e75fd89b8a455675d574a"
	require.Equal(t, expectedRootHashHex, hex.EncodeToString(emptyTrie.RootHash()))
}

// Test_TrieWithEdgeRegister tests whether the root hash of trie with only the most left or
// most right register populated matches the formal specification.
// The expected value is coming from a reference implementation in python and hard-coded here.
func Test_TrieWithEdgeRegister(t *testing.T) {
	// Make new Trie (independently of MForest):
	emptyTrie, err := trie.NewEmptyMTrie(ReferenceImplKeyByteSize, 23, []byte{})
	require.NoError(t, err)

	key := uint2binary(0)
	value := uint2binary(12345)
	leftPopulatedTrie, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, [][]byte{key}, [][]byte{value})
	require.NoError(t, err)
	expectedRootHashHex := "ff472d38a97b3b1786c4dfffa0005370aa3c16805d342ed7618876df7101f760"
	require.Equal(t, expectedRootHashHex, hex.EncodeToString(leftPopulatedTrie.RootHash()))

	key = uint2binary(65535)
	value = uint2binary(54321)
	rightPopulatedTrie, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, [][]byte{key}, [][]byte{value})
	require.NoError(t, err)
	expectedRootHashHex = "d1fb1c7c84bcd02205fbc7bdf73ee8e943b8bb4b7db6bcc26ae7af67e507fb8d"
	require.Equal(t, expectedRootHashHex, hex.EncodeToString(rightPopulatedTrie.RootHash()))
}

// Test_TrieWithEdgeRegister tests whether the root hash of trie with only the most left or
// most right register populated matches the formal specification.
// The expected value is coming from a reference implementation in python and hard-coded here.
func Test_TrieWithSingleRegister(t *testing.T) {
	// Make new Trie (independently of MForest):
	emptyTrie, err := trie.NewEmptyMTrie(ReferenceImplKeyByteSize, 23, []byte{})
	require.NoError(t, err)

	key := uint2binary(56809)
	value := uint2binary(12345)
	leftPopulatedTrie, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, [][]byte{key}, [][]byte{value})
	require.NoError(t, err)
	expectedRootHashHex := "45aef7fed0a92c46e9e282e3a618ecc1cc72f4d8ced751ae90ceef0170f75350"
	require.Equal(t, expectedRootHashHex, hex.EncodeToString(leftPopulatedTrie.RootHash()))
}

// Test_TrieWithEdgeRegister tests whether the root hash of trie with only the most left or
// most right register populated matches the formal specification.
// The expected value is coming from a reference implementation in python and hard-coded here.
func Test_TrieWithSingleRegister2(t *testing.T) {
	// Make new Trie (independently of MForest):
	emptyTrie, err := trie.NewEmptyMTrie(ReferenceImplKeyByteSize, 23, []byte{})
	require.NoError(t, err)

	key := uint2binary(56809)
	value := uint2binary(59656)
	leftPopulatedTrie, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, [][]byte{key}, [][]byte{value})
	require.NoError(t, err)
	expectedRootHashHex := "507b30093c0da1d57808de345d2e79882e5dae6ab1d784d8d7cd4a5de4ce0d76"
	require.Equal(t, expectedRootHashHex, hex.EncodeToString(leftPopulatedTrie.RootHash()))
}

// TestUpdateTrie tests whether the root hash trie storing _many_ randomly selected registers
// matches the formal specification.
// The expected value is coming from a reference implementation in python and hard-coded here.
func Test_UpdateTrie(t *testing.T) {
	// Make new Trie (independently of MForest):
	emptyTrie, err := trie.NewEmptyMTrie(ReferenceImplKeyByteSize, 23, []byte{})
	require.NoError(t, err)

	// allocate single random register
	rng := &LinearCongruentialGenerator{seed: 0}
	key := uint2binary(rng.next())
	value := uint2binary(rng.next())
	updatedTrie, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, [][]byte{key}, [][]byte{value})
	require.NoError(t, err)
	expectedRootHashHex := "a8dc0574fdeeaab4b5d3b2a798c19bee5746337a9aea735ebc4dfd97311503c5"
	require.Equal(t, expectedRootHashHex, hex.EncodeToString(updatedTrie.RootHash()))

	// update further 20 random registers
	keys, values := deduplicateRegisterWrites(sampleRandomRegisterWrites(rng, 20))
	updatedTrie, err = trie.NewTrieWithUpdatedRegisters(updatedTrie, keys, values)
	require.NoError(t, err)
	expectedRootHashHex = "c4e025d432bbb5e16eced56cc9883371c9f4149bc837dbc541540bae7295e11f"
	require.Equal(t, expectedRootHashHex, hex.EncodeToString(updatedTrie.RootHash()))

	// update further 500 random registers
	keys, values = deduplicateRegisterWrites(sampleRandomRegisterWrites(rng, 14))
	updatedTrie, err = trie.NewTrieWithUpdatedRegisters(updatedTrie, keys, values)
	require.NoError(t, err)
	expectedRootHashHex = "75c77d7b2d856a3074616525b65488f7422311dace59fd9a40715349a91b5f86"
	require.Equal(t, expectedRootHashHex, hex.EncodeToString(updatedTrie.RootHash()))

	// update further 500 random registers
	keys, values = deduplicateRegisterWrites(sampleRandomRegisterWrites(rng, 1))
	fmt.Printf(hex.EncodeToString(keys[0]) + "\n")
	fmt.Printf(hex.EncodeToString(values[0]) + "\n")
	updatedTrie, err = trie.NewTrieWithUpdatedRegisters(updatedTrie, keys, values)
	require.NoError(t, err)
	expectedRootHashHex = "75c77d7b2d856a3074616525b65488f7422311dace59fd9a40715349a91b5f86"
	require.Equal(t, expectedRootHashHex, hex.EncodeToString(updatedTrie.RootHash()))

	//"a8dc0574fdeeaab4b5d3b2a798c19bee5746337a9aea735ebc4dfd97311503c5"
	//"c4e025d432bbb5e16eced56cc9883371c9f4149bc837dbc541540bae7295e11f"
	//"338614596b677469c7387a1e0c35453b1222c1c81ce982d2c587d3bc153934ee"
	//"fdc2fa9a15a1079fe635f8dd60411b890f68875f4e08c256da113165da8e589f"
}

// TestUpdateTrie tests whether the root hash of a non-empty trie matches the formal specification.
// The expected value is coming from a reference implementation in python and hard-coded here.
func Test_UpdateTrie2(t *testing.T) {
	// Make new Trie (independently of MForest):
	emptyTrie, err := trie.NewEmptyMTrie(ReferenceImplKeyByteSize, 23, []byte{})
	require.NoError(t, err)

	key := uint2binary(40643)
	value := uint2binary(36474)
	updatedTrie, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, [][]byte{key}, [][]byte{value})
	require.NoError(t, err)

	expectedRootHashHex := "a8dc0574fdeeaab4b5d3b2a798c19bee5746337a9aea735ebc4dfd97311503c5"
	fmt.Printf(hex.EncodeToString(updatedTrie.RootHash()) + "\n")
	require.Equal(t, expectedRootHashHex, hex.EncodeToString(updatedTrie.RootHash()))

}

func Test_Leaf(t *testing.T) {
	key := uint2binary(56809)
	value := uint2binary(59656)

	n := node.NewLeaf(key, value, 0)
	h := n.Hash()
	fmt.Printf("Leaf Hash: %s\n", hex.EncodeToString(h))
	s := ""
	for i := 0; i < 16; i++ {
		d := common.GetDefaultHashForHeight(i)
		right, _ := common.IsBitSet(key, 15-i)
		if right {
			s = "1" + s
			fmt.Printf("%20s: '%s' + '%s' -> ", s, hex.EncodeToString(d), hex.EncodeToString(h))
			h = common.HashInterNode(d, h)
			fmt.Printf("'%s'\n", hex.EncodeToString(h))
		} else {
			s = "0" + s
			fmt.Printf("%20s: '%s' + '%s' -> ", s, hex.EncodeToString(h), hex.EncodeToString(d))
			h = common.HashInterNode(h, d)
			fmt.Printf("'%s'\n", hex.EncodeToString(h))
		}
	}

	fmt.Printf("Target Hash: %s\n", hex.EncodeToString(node.NewLeaf(key, value, 16).Hash()))
}

// TestUpdateTrie tests whether the root hash of a non-empty trie matches the formal specification.
// The expected value is coming from a reference implementation in python and hard-coded here.
func Test_UpdateTrie3(t *testing.T) {
	rng := LinearCongruentialGenerator{}
	for i := 0; i < 10000; i++ {
		fmt.Printf("%d\n", rng.next())
	}
}

func uint2binary(integer uint16) []byte {
	b := make([]byte, ReferenceImplKeyByteSize)
	binary.BigEndian.PutUint16(b, integer)
	return b
}

type LinearCongruentialGenerator struct {
	seed uint64
}

func (rng *LinearCongruentialGenerator) next() uint16 {
	rng.seed = (rng.seed*1140671485 + 12820163) % 65536
	return uint16(rng.seed)
}

// sampleRandomRegisterWrites generates key-value prairs for `number` randomly selected registers;
// caution: registers might repeat
func sampleRandomRegisterWrites(rng *LinearCongruentialGenerator, number uint64) ([][]byte, [][]byte) {
	var i uint64
	keys := make([][]byte, 0, number)
	values := make([][]byte, 0, number)
	for i = 0; i < number; i++ {
		keys = append(keys, uint2binary(rng.next()))
		values = append(values, uint2binary(rng.next()))
	}
	return keys, values
}

// deduplicateRegisterWrites retains only the last register write
func deduplicateRegisterWrites(keys, values [][]byte) ([][]byte, [][]byte) {
	kvPairs := make(map[string]int)
	if len(keys) != len(values) {
		panic("mismatching keys and values")
	}
	for i, key := range keys {
		kvPairs[string(key)] = i
	}
	dedupedKeys := make([][]byte, 0, len(kvPairs))
	dedupedValues := make([][]byte, 0, len(kvPairs))
	for _, idx := range kvPairs {
		dedupedKeys = append(dedupedKeys, keys[idx])
		dedupedValues = append(dedupedValues, values[idx])
	}
	return dedupedKeys, dedupedValues
}
