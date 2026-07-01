package payloadless_test

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"math"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"gotest.tools/assert"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/bitutils"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/ledger/complete/payloadless"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestEmptyTrie tests whether the root hash of an empty trie matches the formal specification.
func Test_EmptyTrie(t *testing.T) {
	// Make new Trie (independently of MForest):
	emptyTrie := payloadless.NewEmptyMTrie()
	rootHash := emptyTrie.RootHash()
	require.Equal(t, ledger.GetDefaultHashForHeight(ledger.NodeMaxHeight), hash.Hash(rootHash))

	// verify root hash
	expectedRootHashHex := "568f4ec740fe3b5de88034cb7b1fbddb41548b068f31aebc8ae9189e429c5749"
	require.Equal(t, expectedRootHashHex, rootHashToString(rootHash))

	// check String() method does not panic:
	_ = emptyTrie.String()
}

// Test_TrieWithLeftRegister tests whether the root hash of trie with only the left-most
// register populated matches the formal specification.
// The expected value is coming from a reference implementation in python and is hard-coded here.
func Test_TrieWithLeftRegister(t *testing.T) {
	// Make new Trie (independently of MForest):
	emptyTrie := payloadless.NewEmptyMTrie()
	path := testutils.PathByUint16LeftPadded(0)
	value := payloadValue(testutils.LightPayload(11, 12345))
	leftPopulatedTrie, maxDepthTouched, err := payloadless.NewTrieWithUpdatedRegisters(emptyTrie, []ledger.Path{path}, [][]byte{value}, true)
	require.NoError(t, err)
	require.Equal(t, uint16(0), maxDepthTouched)
	require.Equal(t, uint64(1), leftPopulatedTrie.AllocatedRegCount())
	expectedRootHashHex := "b30c99cc3e027a6ff463876c638041b1c55316ed935f1b3699e52a2c3e3eaaab"
	require.Equal(t, expectedRootHashHex, rootHashToString(leftPopulatedTrie.RootHash()))
}

// Test_TrieWithRightRegister tests whether the root hash of trie with only the right-most
// register populated matches the formal specification.
// The expected value is coming from a reference implementation in python and is hard-coded here.
func Test_TrieWithRightRegister(t *testing.T) {
	// Make new Trie (independently of MForest):
	emptyTrie := payloadless.NewEmptyMTrie()
	// build a path with all 1s
	var path ledger.Path
	for i := 0; i < len(path); i++ {
		path[i] = uint8(255)
	}
	value := payloadValue(testutils.LightPayload(12346, 54321))
	rightPopulatedTrie, maxDepthTouched, err := payloadless.NewTrieWithUpdatedRegisters(emptyTrie, []ledger.Path{path}, [][]byte{value}, true)
	require.NoError(t, err)
	require.Equal(t, uint16(0), maxDepthTouched)
	require.Equal(t, uint64(1), rightPopulatedTrie.AllocatedRegCount())
	expectedRootHashHex := "4313d22bcabbf21b1cfb833d38f1921f06a91e7198a6672bc68fa24eaaa1a961"
	require.Equal(t, expectedRootHashHex, rootHashToString(rightPopulatedTrie.RootHash()))
}

// Test_TrieWithMiddleRegister tests the root hash of trie holding only a single
// allocated register somewhere in the middle.
// The expected value is coming from a reference implementation in python and is hard-coded here.
func Test_TrieWithMiddleRegister(t *testing.T) {
	// Make new Trie (independently of MForest):
	emptyTrie := payloadless.NewEmptyMTrie()

	path := testutils.PathByUint16LeftPadded(56809)
	value := payloadValue(testutils.LightPayload(12346, 59656))
	leftPopulatedTrie, maxDepthTouched, err := payloadless.NewTrieWithUpdatedRegisters(emptyTrie, []ledger.Path{path}, [][]byte{value}, true)
	require.Equal(t, uint16(0), maxDepthTouched)
	require.Equal(t, uint64(1), leftPopulatedTrie.AllocatedRegCount())
	require.NoError(t, err)
	expectedRootHashHex := "4a29dad0b7ae091a1f035955e0c9aab0692b412f60ae83290b6290d4bf3eb296"
	require.Equal(t, expectedRootHashHex, rootHashToString(leftPopulatedTrie.RootHash()))
}

// Test_TrieWithManyRegisters tests whether the root hash of a trie storing 12001 randomly selected registers
// matches the formal specification.
// The expected value is coming from a reference implementation in python and is hard-coded here.
func Test_TrieWithManyRegisters(t *testing.T) {
	// Make new Trie (independently of MForest):
	emptyTrie := payloadless.NewEmptyMTrie()
	// allocate single random register
	rng := &LinearCongruentialGenerator{seed: 0}
	paths, values := deduplicateWrites(sampleRandomRegisterWrites(rng, 12001))
	updatedTrie, maxDepthTouched, err := payloadless.NewTrieWithUpdatedRegisters(emptyTrie, paths, values, true)
	require.NoError(t, err)
	require.Equal(t, uint16(255), maxDepthTouched)
	require.Equal(t, uint64(12001), updatedTrie.AllocatedRegCount())
	expectedRootHashHex := "74f748dbe563bb5819d6c09a34362a048531fd9647b4b2ea0b6ff43f200198aa"
	require.Equal(t, expectedRootHashHex, rootHashToString(updatedTrie.RootHash()))
}

// Test_FullTrie tests whether the root hash of a trie,
// whose left-most 65536 registers are populated, matches the formal specification.
// The expected value is coming from a reference implementation in python and is hard-coded here.
func Test_FullTrie(t *testing.T) {
	// Make new Trie (independently of MForest):
	emptyTrie := payloadless.NewEmptyMTrie()

	// allocate 65536 left-most registers
	numberRegisters := 65536
	rng := &LinearCongruentialGenerator{seed: 0}
	paths := make([]ledger.Path, 0, numberRegisters)
	values := make([][]byte, 0, numberRegisters)
	for i := 0; i < numberRegisters; i++ {
		paths = append(paths, testutils.PathByUint16LeftPadded(uint16(i)))
		temp := rng.next()
		values = append(values, payloadValue(testutils.LightPayload(temp, temp)))
	}
	updatedTrie, maxDepthTouched, err := payloadless.NewTrieWithUpdatedRegisters(emptyTrie, paths, values, true)
	require.NoError(t, err)
	require.Equal(t, uint16(256), maxDepthTouched)
	require.Equal(t, uint64(numberRegisters), updatedTrie.AllocatedRegCount())
	expectedRootHashHex := "6b3a48d672744f5586c571c47eae32d7a4a3549c1d4fa51a0acfd7b720471de9"
	require.Equal(t, expectedRootHashHex, rootHashToString(updatedTrie.RootHash()))
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
	emptyTrie := payloadless.NewEmptyMTrie()

	// allocate single random register
	rng := &LinearCongruentialGenerator{seed: 0}
	path := testutils.PathByUint16LeftPadded(rng.next())
	temp := rng.next()
	value := payloadValue(testutils.LightPayload(temp, temp))
	updatedTrie, maxDepthTouched, err := payloadless.NewTrieWithUpdatedRegisters(emptyTrie, []ledger.Path{path}, [][]byte{value}, true)
	require.NoError(t, err)
	require.Equal(t, uint16(0), maxDepthTouched)
	require.Equal(t, uint64(1), updatedTrie.AllocatedRegCount())
	expectedRootHashHex := "08db9aeed2b9fcc66b63204a26a4c28652e44e3035bd87ba0ed632a227b3f6dd"
	require.Equal(t, expectedRootHashHex, rootHashToString(updatedTrie.RootHash()))

	var paths []ledger.Path
	var values [][]byte
	parentTrieRegCount := updatedTrie.AllocatedRegCount()
	for r := 0; r < 20; r++ {
		paths, values = deduplicateWrites(sampleRandomRegisterWrites(rng, r*100))
		updatedTrie, maxDepthTouched, err = payloadless.NewTrieWithUpdatedRegisters(updatedTrie, paths, values, true)
		require.NoError(t, err)
		switch r {
		case 0:
			require.Equal(t, uint16(0), maxDepthTouched)
		case 1:
			require.Equal(t, uint16(254), maxDepthTouched)
		default:
			require.Equal(t, uint16(255), maxDepthTouched)
		}
		require.Equal(t, parentTrieRegCount+uint64(len(values)), updatedTrie.AllocatedRegCount())
		require.Equal(t, expectedRootHashes[r], rootHashToString(updatedTrie.RootHash()))

		parentTrieRegCount = updatedTrie.AllocatedRegCount()
	}
	// update with the same registers with the same values
	newTrie, maxDepthTouched, err := payloadless.NewTrieWithUpdatedRegisters(updatedTrie, paths, values, true)
	require.NoError(t, err)
	require.Equal(t, uint16(255), maxDepthTouched)
	require.Equal(t, updatedTrie.AllocatedRegCount(), newTrie.AllocatedRegCount())
	require.Equal(t, expectedRootHashes[19], rootHashToString(updatedTrie.RootHash()))
	// check the root node pointers are equal
	require.True(t, updatedTrie.RootNode() == newTrie.RootNode())
}

// Test_UnallocateRegisters tests whether unallocating registers matches the formal specification.
// Unallocating here means, to set the stored register value to an empty byte slice.
// The expected value is coming from a reference implementation in python and is hard-coded here.
func Test_UnallocateRegisters(t *testing.T) {
	rng := &LinearCongruentialGenerator{seed: 0}
	emptyTrie := payloadless.NewEmptyMTrie()

	// we first draw 99 random key-value pairs that will be first allocated and later unallocated:
	paths1, values1 := deduplicateWrites(sampleRandomRegisterWrites(rng, 99))
	updatedTrie, maxDepthTouched, err := payloadless.NewTrieWithUpdatedRegisters(emptyTrie, paths1, values1, true)
	require.NoError(t, err)
	require.Equal(t, uint16(254), maxDepthTouched)
	require.Equal(t, uint64(len(values1)), updatedTrie.AllocatedRegCount())

	// we then write an additional 117 registers
	paths2, values2 := deduplicateWrites(sampleRandomRegisterWrites(rng, 117))
	updatedTrie, maxDepthTouched, err = payloadless.NewTrieWithUpdatedRegisters(updatedTrie, paths2, values2, true)
	require.Equal(t, uint16(254), maxDepthTouched)
	require.Equal(t, uint64(len(values1)+len(values2)), updatedTrie.AllocatedRegCount())
	require.NoError(t, err)

	// and now we override the first 99 registers with default values, i.e. unallocate them
	emptyValues0 := make([][]byte, len(values1))
	updatedTrie, maxDepthTouched, err = payloadless.NewTrieWithUpdatedRegisters(updatedTrie, paths1, emptyValues0, true)
	require.Equal(t, uint16(254), maxDepthTouched)
	require.Equal(t, uint64(len(values2)), updatedTrie.AllocatedRegCount())
	require.NoError(t, err)

	// this should be identical to the first 99 registers never been written
	expectedRootHashHex := "d81e27a93f2bef058395f70e00fb5d3c8e426e22b3391d048b34017e1ecb483e"
	comparisonTrie, maxDepthTouched, err := payloadless.NewTrieWithUpdatedRegisters(emptyTrie, paths2, values2, true)
	require.NoError(t, err)
	require.Equal(t, uint16(254), maxDepthTouched)
	require.Equal(t, uint64(len(values2)), comparisonTrie.AllocatedRegCount())
	require.Equal(t, expectedRootHashHex, rootHashToString(comparisonTrie.RootHash()))
	require.Equal(t, expectedRootHashHex, rootHashToString(updatedTrie.RootHash()))
}

// simple Linear congruential RNG
// https://en.wikipedia.org/wiki/Linear_congruential_generator
// with configuration for 16bit output used by Microsoft Visual Basic 6 and earlier
type LinearCongruentialGenerator struct {
	seed uint64
}

func (rng *LinearCongruentialGenerator) next() uint16 {
	rng.seed = (rng.seed*1140671485 + 12820163) % 65536
	return uint16(rng.seed)
}

// payloadValue extracts the raw value bytes from a *ledger.Payload, which is
// what the payloadless trie hashes.
func payloadValue(p *ledger.Payload) []byte {
	return []byte(p.Value())
}

// sampleRandomRegisterWrites generates path-value tuples for `number` randomly selected registers;
// caution: registers might repeat
func sampleRandomRegisterWrites(rng *LinearCongruentialGenerator, number int) ([]ledger.Path, [][]byte) {
	paths := make([]ledger.Path, 0, number)
	values := make([][]byte, 0, number)
	for i := 0; i < number; i++ {
		path := testutils.PathByUint16LeftPadded(rng.next())
		paths = append(paths, path)
		t := rng.next()
		values = append(values, payloadValue(testutils.LightPayload(t, t)))
	}
	return paths, values
}

// sampleRandomRegisterWritesWithPrefix generates path-value tuples for `number` randomly selected registers;
// each path is starting with the specified `prefix` and is filled to the full length with random bytes
// caution: register paths might repeat
func sampleRandomRegisterWritesWithPrefix(rng *LinearCongruentialGenerator, number int, prefix []byte) ([]ledger.Path, [][]byte) {
	prefixLen := len(prefix)
	if prefixLen >= hash.HashLen {
		panic("prefix must be shorter than full path length, so there is some space left for random path segment")
	}

	paths := make([]ledger.Path, 0, number)
	values := make([][]byte, 0, number)
	nextRandomBytes := make([]byte, 2)
	nextRandomByteIndex := 2 // index of next unused byte in nextRandomBytes; if value is >= 2, we need to generate new random bytes
	for i := 0; i < number; i++ {
		var p ledger.Path
		copy(p[:prefixLen], prefix)
		for b := prefixLen; b < hash.HashLen; b++ {
			if nextRandomByteIndex >= 2 {
				// pre-generate next 2 bytes
				binary.BigEndian.PutUint16(nextRandomBytes, rng.next())
				nextRandomByteIndex = 0
			}
			p[b] = nextRandomBytes[nextRandomByteIndex]
			nextRandomByteIndex++
		}
		paths = append(paths, p)

		t := rng.next()
		values = append(values, payloadValue(testutils.LightPayload(t, t)))
	}
	return paths, values
}

// deduplicateWrites retains only the last register write
func deduplicateWrites(paths []ledger.Path, values [][]byte) ([]ledger.Path, [][]byte) {
	mapping := make(map[ledger.Path]int)
	if len(paths) != len(values) {
		panic("size mismatch (paths and values)")
	}
	for i, path := range paths {
		// we override the latest in the slice
		mapping[path] = i
	}
	dedupedPaths := make([]ledger.Path, 0, len(mapping))
	dedupedValues := make([][]byte, 0, len(mapping))
	for path := range mapping {
		dedupedPaths = append(dedupedPaths, path)
		dedupedValues = append(dedupedValues, values[mapping[path]])
	}
	return dedupedPaths, dedupedValues
}

func TestSplitByPath(t *testing.T) {
	rand := unittest.GetPRG(t)

	const pathsNumber = 100
	const redundantPaths = 10
	const pathsSize = 32
	randomIndex := rand.Intn(pathsSize)

	// create path slice with redundant paths
	paths := make([]ledger.Path, 0, pathsNumber)
	for i := 0; i < pathsNumber-redundantPaths; i++ {
		var p ledger.Path
		_, err := rand.Read(p[:])
		require.NoError(t, err)
		paths = append(paths, p)
	}
	for i := 0; i < redundantPaths; i++ {
		paths = append(paths, paths[i])
	}

	// save a sorted paths copy for later check
	sortedPaths := make([]ledger.Path, len(paths))
	copy(sortedPaths, paths)
	sort.Slice(sortedPaths, func(i, j int) bool {
		return bytes.Compare(sortedPaths[i][:], sortedPaths[j][:]) < 0
	})

	// split paths
	index := payloadless.SplitPaths(paths, randomIndex)

	// check correctness
	for i := 0; i < index; i++ {
		assert.Equal(t, bitutils.ReadBit(paths[i][:], randomIndex), 0)
	}
	for i := index; i < len(paths); i++ {
		assert.Equal(t, bitutils.ReadBit(paths[i][:], randomIndex), 1)
	}

	// check the multi-set didn't change
	sort.Slice(paths, func(i, j int) bool {
		return bytes.Compare(paths[i][:], paths[j][:]) < 0
	})
	for i := index; i < len(paths); i++ {
		assert.Equal(t, paths[i], sortedPaths[i])
	}
}

// Test_DifferentiateEmptyVsLeaf tests correct behaviour for a very specific edge case for pruning:
//   - By convention, a node in the trie is a leaf if both children are nil.
//   - Therefore, we consider a completely unallocated subtrie also as a potential leaf.
//
// An edge case can now arise when unallocating a previously allocated leaf (see vertex '■' in the illustration below):
//
//   - Before the update, both children of the leaf are nil (because it is a leaf)
//   - After the update-algorithm updated the sub-Trie with root ■, both children of the updated vertex are
//     also nil. But the sub-trie has now changed: the register previously represented by ■ is now gone.
//
// This case must be explicitly handled by the update algorithm:
//
//   - (i)  If the vertex is an interim node, i.e. it had at least one child, it is legal to re-use the vertex if neither
//     of its child-subtries were affected by the update.
//   - (ii) If the vertex is a leaf, only checking that neither child-subtries were affected by the update is insufficient.
//     This is because the register the leaf represents might itself be affected by the update.
//
// Condition (ii) is particularly subtle, if there are register updates in the subtrie of the leaf:
//
//   - From an API perspective, it is a legal operation to set an unallocated register to nil (essentially a no-op).
//
//   - Though, the Trie-update algorithm only realizes that the register is already unallocated, once it traverses
//     into the respective sub-trie. When bubbling up from the recursion, nothing has changed in the children of ■
//     but the vertex ■ itself has changed from an allocated leaf register to an unallocated register.
func Test_DifferentiateEmptyVsLeaf(t *testing.T) {
	//           ⋮  commonPrefix29bytes 101 ....
	//           o
	//          / \
	//        /    \
	//       /      \
	//      ■        o
	//    Left      / \
	//  SubTrie     ⋮  ⋮
	//             Right
	//            SubTrie
	// Left Sub-Trie (■) is a single compactified leaf
	// Right Sub-Trie contains multiple (18) allocated registers

	commonPrefix29bytes := "a0115ce6d49ffe0c9c3d8382826bbec896a9555e4c7720c45b558e7a9e"
	leftSubTriePrefix, _ := hex.DecodeString(commonPrefix29bytes + "0")  // in total 30 bytes
	rightSubTriePrefix, _ := hex.DecodeString(commonPrefix29bytes + "1") // in total 30 bytes

	rng := &LinearCongruentialGenerator{seed: 0}
	leftSubTriePath, leftSubTrieValue := sampleRandomRegisterWritesWithPrefix(rng, 1, leftSubTriePrefix)
	rightSubTriePath, rightSubTrieValue := deduplicateWrites(sampleRandomRegisterWritesWithPrefix(rng, 18, rightSubTriePrefix))

	// initialize Trie to the depicted state
	paths := append(leftSubTriePath, rightSubTriePath...)
	values := append(leftSubTrieValue, rightSubTrieValue...)
	startTrie, maxDepthTouched, err := payloadless.NewTrieWithUpdatedRegisters(payloadless.NewEmptyMTrie(), paths, values, true)
	require.NoError(t, err)
	require.Equal(t, uint16(241), maxDepthTouched)
	require.Equal(t, uint64(len(values)), startTrie.AllocatedRegCount())
	expectedRootHashHex := "8cf6659db0af7626ab0991e2a49019353d549aa4a8c4be1b33e8953d1a9b7fdd"
	require.Equal(t, expectedRootHashHex, rootHashToString(startTrie.RootHash()))

	// Register update:
	//  * de-allocate the compactified leaf (■), i.e. set its value to nil.
	//  * also set a previously already unallocated register to nil as well
	unallocatedRegister := leftSubTriePath[0]            // copy path to leaf and modify it (next line)
	unallocatedRegister[len(unallocatedRegister)-1] ^= 1 // path differs only in the last byte, i.e. register is also in the left Sub-Trie
	updatedPaths := append(leftSubTriePath, unallocatedRegister)
	updatedValues := [][]byte{nil, nil}
	updatedTrie, maxDepthTouched, err := payloadless.NewTrieWithUpdatedRegisters(startTrie, updatedPaths, updatedValues, true)
	require.Equal(t, uint16(256), maxDepthTouched)
	require.Equal(t, uint64(len(rightSubTrieValue)), updatedTrie.AllocatedRegCount())
	require.NoError(t, err)

	// The updated trie should equal to a trie containing only the right sub-Trie
	expectedUpdatedRootHashHex := "576e12a7ef5c760d5cc808ce50e9297919b21b87656b0cc0d9fe8a1a589cf42c"
	require.Equal(t, expectedUpdatedRootHashHex, rootHashToString(updatedTrie.RootHash()))
	referenceTrie, maxDepthTouched, err := payloadless.NewTrieWithUpdatedRegisters(payloadless.NewEmptyMTrie(), rightSubTriePath, rightSubTrieValue, true)
	require.NoError(t, err)
	require.Equal(t, uint16(241), maxDepthTouched)
	require.Equal(t, uint64(len(rightSubTrieValue)), referenceTrie.AllocatedRegCount())
	require.Equal(t, expectedUpdatedRootHashHex, rootHashToString(referenceTrie.RootHash()))
}

func Test_Pruning(t *testing.T) {
	rand := unittest.GetPRG(t)
	emptyTrie := payloadless.NewEmptyMTrie()

	path1 := testutils.PathByUint16(1 << 12)       // 000100...
	path2 := testutils.PathByUint16(1 << 13)       // 001000...
	path4 := testutils.PathByUint16(1<<14 + 1<<13) // 01100...
	path6 := testutils.PathByUint16(1 << 15)       // 1000...

	value1 := payloadValue(testutils.LightPayload(2, 1))
	value2 := payloadValue(testutils.LightPayload(2, 2))
	value4 := payloadValue(testutils.LightPayload(2, 4))
	value6 := payloadValue(testutils.LightPayload(2, 6))

	paths := []ledger.Path{path1, path2, path4, path6}
	values := [][]byte{value1, value2, value4, value6}

	//                    n7
	//                   / \
	//                 /     \
	//             n5         n6 (path6/value6) // 1000
	//            /  \
	//          /      \
	//         /         \
	//        n3          n4 (path4/value4) // 01100...
	//      /     \
	//    /          \
	//  /              \
	// n1 (path1,       n2 (path2)
	//     value1)         value2)

	baseTrie, maxDepthTouched, err := payloadless.NewTrieWithUpdatedRegisters(emptyTrie, paths, values, true)
	require.NoError(t, err)
	require.Equal(t, uint16(3), maxDepthTouched)
	require.Equal(t, uint64(len(values)), baseTrie.AllocatedRegCount())

	t.Run("leaf update with pruning test", func(t *testing.T) {
		expectedRegCount := baseTrie.AllocatedRegCount() - 1

		trie1, maxDepthTouched, err := payloadless.NewTrieWithUpdatedRegisters(baseTrie, []ledger.Path{path1}, [][]byte{nil}, false)
		require.NoError(t, err)
		require.Equal(t, uint16(3), maxDepthTouched)
		require.Equal(t, expectedRegCount, trie1.AllocatedRegCount())

		trie1withpruning, maxDepthTouched, err := payloadless.NewTrieWithUpdatedRegisters(baseTrie, []ledger.Path{path1}, [][]byte{nil}, true)
		require.NoError(t, err)
		require.Equal(t, uint16(3), maxDepthTouched)
		require.Equal(t, expectedRegCount, trie1withpruning.AllocatedRegCount())
		require.True(t, trie1withpruning.RootNode().VerifyCachedHash())

		// after pruning
		//                    n7
		//                   / \
		//                 /     \
		//             n5         n6 (path6/value6) // 1000
		//            /  \
		//          /      \
		//         /         \
		//     n3 (path2       n4 (path4
		//        /value2)        /value4) // 01100...
		require.Equal(t, trie1.RootHash(), trie1withpruning.RootHash())
	})

	t.Run("leaf update with two level pruning test", func(t *testing.T) {
		expectedRegCount := baseTrie.AllocatedRegCount() - 1

		// setting path4 to zero from baseTrie
		trie2, maxDepthTouched, err := payloadless.NewTrieWithUpdatedRegisters(baseTrie, []ledger.Path{path4}, [][]byte{nil}, false)
		require.NoError(t, err)
		require.Equal(t, uint16(2), maxDepthTouched)
		require.Equal(t, expectedRegCount, trie2.AllocatedRegCount())

		// pruning is not activated here because n3 is not a leaf node
		trie2withpruning, maxDepthTouched, err := payloadless.NewTrieWithUpdatedRegisters(baseTrie, []ledger.Path{path4}, [][]byte{nil}, true)
		require.NoError(t, err)
		require.Equal(t, uint16(2), maxDepthTouched)
		require.Equal(t, expectedRegCount, trie2withpruning.AllocatedRegCount())
		require.True(t, trie2withpruning.RootNode().VerifyCachedHash())

		require.Equal(t, trie2.RootHash(), trie2withpruning.RootHash())

		// now setting path2 to zero should do the pruning for two levels
		expectedRegCount -= 1

		trie22, maxDepthTouched, err := payloadless.NewTrieWithUpdatedRegisters(trie2, []ledger.Path{path2}, [][]byte{nil}, false)
		require.NoError(t, err)
		require.Equal(t, uint16(3), maxDepthTouched)
		require.Equal(t, expectedRegCount, trie22.AllocatedRegCount())

		trie22withpruning, maxDepthTouched, err := payloadless.NewTrieWithUpdatedRegisters(trie2withpruning, []ledger.Path{path2}, [][]byte{nil}, true)
		require.NoError(t, err)
		require.Equal(t, uint16(3), maxDepthTouched)
		require.Equal(t, expectedRegCount, trie22withpruning.AllocatedRegCount())

		// after pruning
		//                     n7
		//                   /   \
		//                 /       \
		//             n5 (path1,   n6 (path6/value6) // 1000
		//                 /value1)

		require.Equal(t, trie22.RootHash(), trie22withpruning.RootHash())
		require.True(t, trie22withpruning.RootNode().VerifyCachedHash())

	})

	t.Run("several updates at the same time", func(t *testing.T) {
		// setting path4 to zero from baseTrie
		trie3, maxDepthTouched, err := payloadless.NewTrieWithUpdatedRegisters(baseTrie, []ledger.Path{path2, path4, path6}, [][]byte{nil, nil, nil}, false)
		require.NoError(t, err)
		require.Equal(t, uint16(3), maxDepthTouched)
		require.Equal(t, uint64(1), trie3.AllocatedRegCount())

		// this should prune two levels
		trie3withpruning, maxDepthTouched, err := payloadless.NewTrieWithUpdatedRegisters(baseTrie, []ledger.Path{path2, path4, path6}, [][]byte{nil, nil, nil}, true)
		require.NoError(t, err)
		require.Equal(t, uint16(3), maxDepthTouched)
		require.Equal(t, uint64(1), trie3withpruning.AllocatedRegCount())

		// after pruning
		//       n7  (path1/value1)
		require.Equal(t, trie3.RootHash(), trie3withpruning.RootHash())
		require.True(t, trie3withpruning.RootNode().VerifyCachedHash())
	})

	t.Run("smoke testing trie pruning", func(t *testing.T) {
		unittest.SkipUnless(t, unittest.TEST_LONG_RUNNING, "skipping trie pruning smoke testing as its not needed to always run")

		numberOfSteps := 1000
		numberOfUpdates := 750
		numberOfRemovals := 750

		var err error
		activeTrie := payloadless.NewEmptyMTrie()
		activeTrieWithPruning := payloadless.NewEmptyMTrie()
		allPaths := make(map[ledger.Path][]byte)
		var maxDepthTouched, maxDepthTouchedWithPruning uint16
		var parentTrieRegCount uint64

		for step := 0; step < numberOfSteps; step++ {

			updatePaths := make([]ledger.Path, 0)
			updateValues := make([][]byte, 0)

			var expectedRegCountDelta int64

			for i := 0; i < numberOfUpdates; {
				var path ledger.Path
				_, err := rand.Read(path[:])
				require.NoError(t, err)
				// deduplicate
				if _, found := allPaths[path]; !found {
					value := payloadValue(testutils.RandomPayload(1, 100))
					updatePaths = append(updatePaths, path)
					updateValues = append(updateValues, value)
					expectedRegCountDelta++
					i++
				}
			}

			i := 0
			samplesNeeded := int(math.Min(float64(numberOfRemovals), float64(len(allPaths))))
			for p := range allPaths {
				updatePaths = append(updatePaths, p)
				updateValues = append(updateValues, nil)
				expectedRegCountDelta--
				delete(allPaths, p)
				i++
				if i > samplesNeeded {
					break
				}
			}

			// only set it for the updates
			for i := 0; i < numberOfUpdates; i++ {
				allPaths[updatePaths[i]] = updateValues[i]
			}

			activeTrie, maxDepthTouched, err = payloadless.NewTrieWithUpdatedRegisters(activeTrie, updatePaths, updateValues, false)
			require.NoError(t, err)
			require.Equal(t, uint64(int64(parentTrieRegCount)+expectedRegCountDelta), activeTrie.AllocatedRegCount())

			activeTrieWithPruning, maxDepthTouchedWithPruning, err = payloadless.NewTrieWithUpdatedRegisters(activeTrieWithPruning, updatePaths, updateValues, true)
			require.NoError(t, err)
			require.True(t, maxDepthTouched >= maxDepthTouchedWithPruning)
			require.Equal(t, uint64(int64(parentTrieRegCount)+expectedRegCountDelta), activeTrieWithPruning.AllocatedRegCount())

			require.Equal(t, activeTrie.RootHash(), activeTrieWithPruning.RootHash())

			parentTrieRegCount = activeTrie.AllocatedRegCount()

			// fetch all leaf hashes and compare
			queryPaths := make([]ledger.Path, 0)
			for path := range allPaths {
				queryPaths = append(queryPaths, path)
			}

			leafHashes := activeTrie.UnsafeRead(queryPaths)
			for i, lh := range leafHashes {
				expectedValue := allPaths[queryPaths[i]]
				expectedLeafHash := hash.HashLeaf(hash.Hash(queryPaths[i]), expectedValue)
				require.NotNil(t, lh)
				require.Equal(t, expectedLeafHash, *lh)
			}

			leafHashes = activeTrieWithPruning.UnsafeRead(queryPaths)
			for i, lh := range leafHashes {
				expectedValue := allPaths[queryPaths[i]]
				expectedLeafHash := hash.HashLeaf(hash.Hash(queryPaths[i]), expectedValue)
				require.NotNil(t, lh)
				require.Equal(t, expectedLeafHash, *lh)
			}

		}
	})
}

func rootHashToString(rh ledger.RootHash) string {
	return hex.EncodeToString(rh[:])
}

// TestTrieAllocatedRegCount tests allocated register count for updated trie.
// It tests the following updates with prune flag set to true:
//   - update empty trie with new paths and values
//   - update trie with existing paths and updated values
//   - update trie with new paths and empty values
//   - update trie with existing path and empty value one by one until trie is empty
//
// It also tests the following updates with prune flag set to false:
//   - update trie with existing path and empty value one by one until trie is empty
//   - update trie with removed paths and empty values
//   - update trie with removed paths and non-empty values
func TestTrieAllocatedRegCount(t *testing.T) {

	rng := &LinearCongruentialGenerator{seed: 0}

	// Allocate 255 registers
	numberRegisters := 255
	paths := make([]ledger.Path, numberRegisters)
	values := make([][]byte, numberRegisters)
	for i := 0; i < numberRegisters; i++ {
		var p ledger.Path
		p[0] = byte(i)

		paths[i] = p
		values[i] = payloadValue(testutils.LightPayload(rng.next(), rng.next()))
	}

	// Update trie with registers to test reg count with new registers.
	updatedTrie, maxDepthTouched, err := payloadless.NewTrieWithUpdatedRegisters(payloadless.NewEmptyMTrie(), paths, values, true)
	require.NoError(t, err)
	require.True(t, maxDepthTouched <= 256)
	require.Equal(t, uint64(len(values)), updatedTrie.AllocatedRegCount())

	// Update trie with existing paths and updated values
	// to test reg count with updated registers
	// (old value present and new value present).
	for i := 0; i < len(values); i += 2 {
		values[i] = payloadValue(testutils.LightPayload(rng.next(), rng.next()))
	}

	updatedTrie, maxDepthTouched, err = payloadless.NewTrieWithUpdatedRegisters(updatedTrie, paths, values, true)
	require.NoError(t, err)
	require.True(t, maxDepthTouched <= 256)
	require.Equal(t, uint64(len(values)), updatedTrie.AllocatedRegCount())

	rootHash := updatedTrie.RootHash()

	// Update trie with new paths and empty values
	// to test reg count with new empty registers.
	newPaths := []ledger.Path{}
	newValues := [][]byte{}
	for i := 0; i < len(paths); i++ {
		oldPath := paths[i]

		path1, _ := ledger.ToPath(oldPath[:])
		path1[1] = 1
		value1 := []byte(nil)

		path2, _ := ledger.ToPath(oldPath[:])
		path2[1] = 2
		value2 := []byte(nil)

		newPaths = append(newPaths, oldPath, path1, path2)
		newValues = append(newValues, values[i], value1, value2)
	}

	updatedTrie, maxDepthTouched, err = payloadless.NewTrieWithUpdatedRegisters(updatedTrie, newPaths, newValues, true)
	require.NoError(t, err)
	require.Equal(t, rootHash, updatedTrie.RootHash())
	require.True(t, maxDepthTouched <= 256)
	require.Equal(t, uint64(len(values)), updatedTrie.AllocatedRegCount())

	t.Run("pruning", func(t *testing.T) {
		expectedRegCount := uint64(len(values))

		updatedTrieWithPruning := updatedTrie

		// Remove register one by one to test reg count with empty registers
		// (old value present and new value empty)
		for i := 0; i < len(paths); i++ {
			newPaths := []ledger.Path{paths[i]}
			newValues := [][]byte{nil}

			expectedRegCount--

			updatedTrieWithPruning, maxDepthTouched, err = payloadless.NewTrieWithUpdatedRegisters(updatedTrieWithPruning, newPaths, newValues, true)
			require.NoError(t, err)
			require.True(t, maxDepthTouched <= 256)
			require.Equal(t, expectedRegCount, updatedTrieWithPruning.AllocatedRegCount())
		}

		// After all registered are removed, reg count should be 0.
		require.Equal(t, payloadless.EmptyTrieRootHash(), updatedTrieWithPruning.RootHash())
		require.Equal(t, uint64(0), updatedTrieWithPruning.AllocatedRegCount())
	})

	t.Run("no pruning", func(t *testing.T) {
		expectedRegCount := uint64(len(values))

		updatedTrieNoPruning := updatedTrie

		// Remove register one by one to test reg count with empty registers
		// (old value present and new value empty)
		for i := 0; i < len(paths); i++ {
			newPaths := []ledger.Path{paths[i]}
			newValues := [][]byte{nil}

			expectedRegCount--

			updatedTrieNoPruning, maxDepthTouched, err = payloadless.NewTrieWithUpdatedRegisters(updatedTrieNoPruning, newPaths, newValues, false)
			require.NoError(t, err)
			require.True(t, maxDepthTouched <= 256)
			require.Equal(t, expectedRegCount, updatedTrieNoPruning.AllocatedRegCount())
		}

		// After all registered are removed, reg count should be 0.
		require.Equal(t, payloadless.EmptyTrieRootHash(), updatedTrieNoPruning.RootHash())
		require.Equal(t, uint64(0), updatedTrieNoPruning.AllocatedRegCount())

		// Update with removed paths and empty values
		// (old value empty and new value empty)
		newValues := make([][]byte, len(paths))

		updatedTrieNoPruning, maxDepthTouched, err = payloadless.NewTrieWithUpdatedRegisters(updatedTrieNoPruning, paths, newValues, false)
		require.NoError(t, err)
		require.True(t, maxDepthTouched <= 256)
		require.Equal(t, payloadless.EmptyTrieRootHash(), updatedTrieNoPruning.RootHash())
		require.Equal(t, uint64(0), updatedTrieNoPruning.AllocatedRegCount())

		// Update with removed paths and non-empty values
		// (old value empty and new value present)
		updatedTrieNoPruning, maxDepthTouched, err = payloadless.NewTrieWithUpdatedRegisters(updatedTrieNoPruning, paths, values, false)
		require.NoError(t, err)
		require.Equal(t, rootHash, updatedTrie.RootHash())
		require.True(t, maxDepthTouched <= 256)
		require.Equal(t, uint64(len(values)), updatedTrieNoPruning.AllocatedRegCount())
	})
}

// TestTrieAllocatedRegCountWithMixedPruneFlag tests allocated register count
// for updated trie with mixed pruning flag.
// It tests the following updates:
//   - step 1 : update empty trie with new paths and values (255 allocated registers)
//   - step 2 : remove a value without pruning (254 allocated registers)
//   - step 3a: remove previously removed value with pruning (254 allocated registers)
//   - step 3b: update trie from step 2 with a new value (sibling of removed value)
//     with pruning (255 allocated registers)
func TestTrieAllocatedRegCountWithMixedPruneFlag(t *testing.T) {
	rng := &LinearCongruentialGenerator{seed: 0}

	// Allocate 255 registers
	numberRegisters := 255
	paths := make([]ledger.Path, numberRegisters)
	values := make([][]byte, numberRegisters)
	for i := 0; i < numberRegisters; i++ {
		var p ledger.Path
		p[0] = byte(i)

		paths[i] = p
		values[i] = payloadValue(testutils.LightPayload(rng.next(), rng.next()))
	}

	expectedAllocatedRegCount := uint64(len(values))

	// Update trie with registers to test reg count with new registers.
	baseTrie, maxDepthTouched, err := payloadless.NewTrieWithUpdatedRegisters(payloadless.NewEmptyMTrie(), paths, values, true)
	require.NoError(t, err)
	require.True(t, maxDepthTouched <= 256)
	require.Equal(t, expectedAllocatedRegCount, baseTrie.AllocatedRegCount())

	// Remove one value without pruning
	expectedAllocatedRegCount--

	removePaths := []ledger.Path{paths[0]}
	removeValues := [][]byte{nil}
	unprunedTrie, maxDepthTouched, err := payloadless.NewTrieWithUpdatedRegisters(baseTrie, removePaths, removeValues, false)
	require.NoError(t, err)
	require.True(t, maxDepthTouched <= 256)
	require.Equal(t, expectedAllocatedRegCount, unprunedTrie.AllocatedRegCount())

	// Remove the same value (no affect) from unprunedTrie with pruning
	// expected reg count remains unchanged.
	updatedTrie, maxDepthTouched, err := payloadless.NewTrieWithUpdatedRegisters(unprunedTrie, removePaths, removeValues, true)
	require.NoError(t, err)
	require.True(t, maxDepthTouched <= 256)
	require.Equal(t, expectedAllocatedRegCount, updatedTrie.AllocatedRegCount())

	// Add sibling of removed path from unprunedTrie with pruning
	newPath := paths[0]
	bitutils.SetBit(newPath[:], ledger.PathLen*8-1)
	newPaths := []ledger.Path{newPath}
	newValues := [][]byte{payloadValue(testutils.LightPayload(rng.next(), rng.next()))}

	// expected reg count is incremented.
	expectedAllocatedRegCount++

	updatedTrie, maxDepthTouched, err = payloadless.NewTrieWithUpdatedRegisters(unprunedTrie, newPaths, newValues, true)
	require.NoError(t, err)
	require.True(t, maxDepthTouched <= 256)
	require.Equal(t, expectedAllocatedRegCount, updatedTrie.AllocatedRegCount())
}

// TestReadSingleLeafHash tests reading a single leaf hash of existent/non-existent path
// for trie of different layouts.
func TestReadSingleLeafHash(t *testing.T) {

	emptyTrie := payloadless.NewEmptyMTrie()

	// Test reading leaf hash in empty trie
	t.Run("empty trie", func(t *testing.T) {
		savedRootHash := emptyTrie.RootHash()

		path := testutils.PathByUint16LeftPadded(0)
		leafHash := emptyTrie.ReadSingleLeafHash(path)
		require.Nil(t, leafHash)
		require.Equal(t, savedRootHash, emptyTrie.RootHash())
	})

	// Test reading leaf hash for existent/non-existent path
	// in trie with compact leaf as root node.
	t.Run("compact leaf as root", func(t *testing.T) {
		path1 := testutils.PathByUint16LeftPadded(0)
		value1 := payloadValue(testutils.RandomPayload(1, 100))

		paths := []ledger.Path{path1}
		values := [][]byte{value1}

		newTrie, maxDepthTouched, err := payloadless.NewTrieWithUpdatedRegisters(emptyTrie, paths, values, true)
		require.NoError(t, err)
		require.Equal(t, uint16(0), maxDepthTouched)

		savedRootHash := newTrie.RootHash()

		// Get leaf hash for existent path
		retLeafHash := newTrie.ReadSingleLeafHash(path1)
		require.NotNil(t, retLeafHash)
		expectedLeafHash := hash.HashLeaf(hash.Hash(path1), value1)
		require.Equal(t, expectedLeafHash, *retLeafHash)
		require.Equal(t, savedRootHash, newTrie.RootHash())

		// Get leaf hash for non-existent path
		path2 := testutils.PathByUint16LeftPadded(1)
		retLeafHash = newTrie.ReadSingleLeafHash(path2)
		require.Nil(t, retLeafHash)
		require.Equal(t, savedRootHash, newTrie.RootHash())
	})

	// Test reading leaf hash for existent/non-existent path in an unpruned trie.
	t.Run("trie", func(t *testing.T) {
		path1 := testutils.PathByUint16(1 << 12) // 000100...
		path2 := testutils.PathByUint16(1 << 13) // 001000...
		path3 := testutils.PathByUint16(1 << 14) // 010000...

		value1 := payloadValue(testutils.RandomPayload(1, 100))
		value2 := payloadValue(testutils.RandomPayload(1, 100))
		var value3 []byte // empty

		paths := []ledger.Path{path1, path2, path3}
		values := [][]byte{value1, value2, value3}

		// Create an unpruned trie with 3 leaf nodes (n1, n2, n3).
		newTrie, maxDepthTouched, err := payloadless.NewTrieWithUpdatedRegisters(emptyTrie, paths, values, false)
		require.NoError(t, err)
		require.Equal(t, uint16(3), maxDepthTouched)

		savedRootHash := newTrie.RootHash()

		//                  n5
		//                 /
		//                /
		//              n4
		//             /  \
		//            /    \
		//           n3      n3 (path3/
		//        /     \        value3)
		//      /         \
		//   n1 (path1/     n2 (path2/
		//       value1)        value2)
		//

		// Test reading leaf hash for all possible paths for the first 4 bits.
		for i := 0; i < 16; i++ {
			path := testutils.PathByUint16(uint16(i << 12))

			retLeafHash := newTrie.ReadSingleLeafHash(path)
			require.Equal(t, savedRootHash, newTrie.RootHash())
			switch path {
			case path1:
				require.NotNil(t, retLeafHash)
				expected := hash.HashLeaf(hash.Hash(path1), value1)
				require.Equal(t, expected, *retLeafHash)
			case path2:
				require.NotNil(t, retLeafHash)
				expected := hash.HashLeaf(hash.Hash(path2), value2)
				require.Equal(t, expected, *retLeafHash)
			default:
				require.Nil(t, retLeafHash)
			}
		}
	})
}
