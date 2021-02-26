package trie_test

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/utils"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
)

const (
	// ReferenceImplPathByteSize is the path length in reference implementation: 32 bytes.
	// Please do NOT CHANGE.
	ReferenceImplPathByteSize = 32
)

// TestEmptyTrie tests whether the root hash of an empty trie matches the formal specification.
// The expected value is coming from a reference implementation in python and is hard-coded here.
func Test_EmptyTrie(t *testing.T) {
	// Make new Trie (independently of MForest):
	emptyTrie, err := trie.NewEmptyMTrie(ReferenceImplPathByteSize)
	require.NoError(t, err)

	expectedRootHashHex := "568f4ec740fe3b5de88034cb7b1fbddb41548b068f31aebc8ae9189e429c5749"
	require.Equal(t, expectedRootHashHex, hex.EncodeToString(emptyTrie.RootHash()))
}

// Test_TrieWithLeftRegister tests whether the root hash of trie with only the left-most
// register populated matches the formal specification.
// The expected value is coming from a reference implementation in python and is hard-coded here.
func Test_TrieWithLeftRegister(t *testing.T) {
	// Make new Trie (independently of MForest):
	emptyTrie, err := trie.NewEmptyMTrie(ReferenceImplPathByteSize)
	require.NoError(t, err)

	path := utils.PathByUint16LeftPadded(0)
	payload := utils.LightPayload(11, 12345)
	leftPopulatedTrie, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, []ledger.Path{path}, []ledger.Payload{*payload})
	require.NoError(t, err)
	expectedRootHashHex := "b30c99cc3e027a6ff463876c638041b1c55316ed935f1b3699e52a2c3e3eaaab"
	require.Equal(t, expectedRootHashHex, hex.EncodeToString(leftPopulatedTrie.RootHash()))
}

// Test_TrieWithRightRegister tests whether the root hash of trie with only the right-most
// register populated matches the formal specification.
// The expected value is coming from a reference implementation in python and is hard-coded here.
func Test_TrieWithRightRegister(t *testing.T) {
	// Make new Trie (independently of MForest):
	emptyTrie, err := trie.NewEmptyMTrie(ReferenceImplPathByteSize)
	require.NoError(t, err)

	path := utils.PathByUint16LeftPadded(65535)
	payload := utils.LightPayload(12346, 54321)
	rightPopulatedTrie, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, []ledger.Path{path}, []ledger.Payload{*payload})
	require.NoError(t, err)
	expectedRootHashHex := "9555f8b5aa449ee47e656544234909fe93637a192cd6c683de02b9d8b7c67fa7"
	require.Equal(t, expectedRootHashHex, hex.EncodeToString(rightPopulatedTrie.RootHash()))
}

// // Test_TrieWithMiddleRegister tests the root hash of trie holding only a single
// // allocated register somewhere in the middle.
// // The expected value is coming from a reference implementation in python and is hard-coded here.
func Test_TrieWithMiddleRegister(t *testing.T) {
	// Make new Trie (independently of MForest):
	emptyTrie, err := trie.NewEmptyMTrie(ReferenceImplPathByteSize)
	require.NoError(t, err)

	path := utils.PathByUint16LeftPadded(56809)
	payload := utils.LightPayload(12346, 59656)
	leftPopulatedTrie, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, []ledger.Path{path}, []ledger.Payload{*payload})
	require.NoError(t, err)
	expectedRootHashHex := "4a29dad0b7ae091a1f035955e0c9aab0692b412f60ae83290b6290d4bf3eb296"
	require.Equal(t, expectedRootHashHex, hex.EncodeToString(leftPopulatedTrie.RootHash()))
}

// Test_TrieWithManyRegisters tests whether the root hash of a trie storing 12001 randomly selected registers
// matches the formal specification.
// The expected value is coming from a reference implementation in python and is hard-coded here.
func Test_TrieWithManyRegisters(t *testing.T) {
	// Make new Trie (independently of MForest):
	emptyTrie, err := trie.NewEmptyMTrie(ReferenceImplPathByteSize)
	require.NoError(t, err)

	// allocate single random register
	rng := &LinearCongruentialGenerator{seed: 0}
	paths, payloads := deduplicateWrites(sampleRandomRegisterWrites(rng, 12001))
	updatedTrie, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths, payloads)
	require.NoError(t, err)
	expectedRootHashHex := "74f748dbe563bb5819d6c09a34362a048531fd9647b4b2ea0b6ff43f200198aa"
	require.Equal(t, expectedRootHashHex, hex.EncodeToString(updatedTrie.RootHash()))
}

// Test_FullTrie tests whether the root hash of a fully-populated trie storing
// matches the formal specification.
// The expected value is coming from a reference implementation in python and is hard-coded here.
func Test_FullTrie(t *testing.T) {
	// Make new Trie (independently of MForest):
	emptyTrie, err := trie.NewEmptyMTrie(ReferenceImplPathByteSize)
	require.NoError(t, err)

	// allocate single random register
	capacity := 65536
	rng := &LinearCongruentialGenerator{seed: 0}
	paths := make([]ledger.Path, 0, capacity)
	payloads := make([]ledger.Payload, 0, capacity)
	for i := 0; i < capacity; i++ {
		paths = append(paths, utils.PathByUint16LeftPadded(uint16(i)))
		temp := rng.next()
		payload := utils.LightPayload(temp, temp)
		payloads = append(payloads, *payload)
	}
	updatedTrie, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths, payloads)
	require.NoError(t, err)
	expectedRootHashHex := "6b3a48d672744f5586c571c47eae32d7a4a3549c1d4fa51a0acfd7b720471de9"
	require.Equal(t, expectedRootHashHex, hex.EncodeToString(updatedTrie.RootHash()))
}

// TestUpdateTrie tests whether iteratively updating a Trie matches the formal specification.
// The expected root hashes are coming from a reference implementation in python and is hard-coded here.
func Test_UpdateTrie(t *testing.T) {
	expectedRootHashes := []string{
		"08db9aeed2b9fcc66b63204a26a4c28652e44e3035bd87ba0ed632a227b3f6dd",
		"2f4b0f490fa05e5b3bbd43176e367c3e9b64cdb710e45d4508fff11759d7a08e",
		"668811792995cd960e7e343540a360682ac375f7ec5533f774c464cd6b34adc9",
		"169c145eaeda2038a0e409068a12cb26bde5e890115ad1ef624f422007fb2d2a",
		"8f87b503a706d9eaf50873030e0e627850c841cc0cf382187b81ba26cec57588",
		"faacc057336e10e13ff6f5667aefc3ac9d9d390b34ee50391a6f7f305dfdf761",
		"049e035735a13fee09a3c36a7f567daf05baee419ac90ade538108492d80b279",
		"bb8340a9772ab6d6aa4862b23c8bb830da226cdf6f6c26f1e1e850077be600af",
		"8b9b7eb5c489bf4aeffd86d3a215dc045856094d0abe5cf7b4cc3f835d499168",
		"6514743e986f20fcf22a02e50ba352a5bfde50fe949b57b990aeb863cfcd81d1",
		"33c3d386e1c7c707f727fdeb65c52117537d175da9ab3f60a0a576301d20756e",
		"09df0bc6eee9d0f76df05d19b2ac550cde8c4294cd6eafaa1332718bd62e912f",
		"8b1fccbf7d1eca093441305ebff72d3f12b8b7cce5b4f89d6f464fc5df83b0d3",
		"0830e2d015742e284c56075050e94d3ff9618a46f28aa9066379f012e45c05fc",
		"9d95255bb75dddc317deda4e45223aa4a5ac02eaa537dc9e602d6f03fa26d626",
		"74f748dbe563bb5819d6c09a34362a048531fd9647b4b2ea0b6ff43f200198aa",
		"c06903580432a27dee461e9022a6546cb4ddec2f8598c48429e9ba7a96a892da",
		"a117f94e9cc6114e19b7639eaa630304788979cf92037736bbeb23ed1504638a",
		"d382c97020371d8788d4c27971a89f1617f9bbf21c49c922f1b683cc36a4646c",
		"ce633e9ca6329d6984c37a46e0a479bb1841674c2db00970dacfe035882d4aba",
	}

	// Make new Trie (independently of MForest):
	emptyTrie, err := trie.NewEmptyMTrie(ReferenceImplPathByteSize)
	require.NoError(t, err)

	// allocate single random register
	rng := &LinearCongruentialGenerator{seed: 0}
	path := utils.PathByUint16LeftPadded(rng.next())
	temp := rng.next()
	payload := utils.LightPayload(temp, temp)
	updatedTrie, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, []ledger.Path{path}, []ledger.Payload{*payload})
	require.NoError(t, err)
	expectedRootHashHex := "08db9aeed2b9fcc66b63204a26a4c28652e44e3035bd87ba0ed632a227b3f6dd"
	require.Equal(t, expectedRootHashHex, hex.EncodeToString(updatedTrie.RootHash()))

	for r := 0; r < 20; r++ {
		paths, payloads := deduplicateWrites(sampleRandomRegisterWrites(rng, r*100))
		updatedTrie, err = trie.NewTrieWithUpdatedRegisters(updatedTrie, paths, payloads)
		require.NoError(t, err)
		require.Equal(t, expectedRootHashes[r], hex.EncodeToString(updatedTrie.RootHash()))
	}
}

// Test_UnallocateRegisters tests whether unallocating registers matches the formal specification.
// Unallocating here means, to set the stored register value to an empty byte slice
// The expected value is coming from a reference implementation in python and is hard-coded here.
func Test_UnallocateRegisters(t *testing.T) {
	rng := &LinearCongruentialGenerator{seed: 0}
	emptyTrie, err := trie.NewEmptyMTrie(ReferenceImplPathByteSize)
	require.NoError(t, err)

	// we first draw 99 random key-value pairs that will be first allocated and later unallocated:
	paths1, payloads1 := deduplicateWrites(sampleRandomRegisterWrites(rng, 99))
	updatedTrie, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths1, payloads1)
	require.NoError(t, err)

	// we then write an additional 117 registers
	paths2, payloads2 := deduplicateWrites(sampleRandomRegisterWrites(rng, 117))
	updatedTrie, err = trie.NewTrieWithUpdatedRegisters(updatedTrie, paths2, payloads2)
	require.NoError(t, err)

	// and now we override the first 99 registers with default values, i.e. unallocate them
	payloads0 := make([]ledger.Payload, len(payloads1))
	updatedTrie, err = trie.NewTrieWithUpdatedRegisters(updatedTrie, paths1, payloads0)
	require.NoError(t, err)

	// this should be identical to the first 99 registers never been written
	expectedRootHashHex := "d81e27a93f2bef058395f70e00fb5d3c8e426e22b3391d048b34017e1ecb483e"
	comparisionTrie, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths2, payloads2)
	require.NoError(t, err)
	require.Equal(t, expectedRootHashHex, hex.EncodeToString(comparisionTrie.RootHash()))
	require.Equal(t, expectedRootHashHex, hex.EncodeToString(updatedTrie.RootHash()))
}

type LinearCongruentialGenerator struct {
	seed uint64
}

func (rng *LinearCongruentialGenerator) next() uint16 {
	rng.seed = (rng.seed*1140671485 + 12820163) % 65536
	return uint16(rng.seed)
}

// sampleRandomRegisterWrites generates path-payload tuples for `number` randomly selected registers;
// caution: registers might repeat
func sampleRandomRegisterWrites(rng *LinearCongruentialGenerator, number int) ([]ledger.Path, []ledger.Payload) {

	paths := make([]ledger.Path, 0, number)
	payloads := make([]ledger.Payload, 0, number)
	for i := 0; i < number; i++ {
		path := utils.PathByUint16LeftPadded(rng.next())
		paths = append(paths, path)
		t := rng.next()
		payload := utils.LightPayload(t, t)
		payloads = append(payloads, *payload)
	}
	return paths, payloads
}

// deduplicateWrites retains only the last register write
func deduplicateWrites(paths []ledger.Path, payloads []ledger.Payload) ([]ledger.Path, []ledger.Payload) {
	payloadMapping := make(map[string]int)
	if len(paths) != len(payloads) {
		panic("size mismatch (paths and payloads)")
	}
	for i, path := range paths {
		// we override the latest in the slice
		payloadMapping[string(path)] = i
	}
	dedupedPaths := make([]ledger.Path, 0, len(payloadMapping))
	dedupedPayloads := make([]ledger.Payload, 0, len(payloadMapping))
	for path := range payloadMapping {
		dedupedPaths = append(dedupedPaths, []byte(path))
		dedupedPayloads = append(dedupedPayloads, payloads[payloadMapping[path]])
	}
	return dedupedPaths, dedupedPayloads
}
