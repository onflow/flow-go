package trie_test

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"math"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gotest.tools/assert"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/bitutils"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/utils"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestEmptyTrie tests whether the root hash of an empty trie matches the formal specification.
func Test_EmptyTrie(t *testing.T) {
	// Make new Trie (independently of MForest):
	emptyTrie := trie.NewEmptyMTrie()
	rootHash := emptyTrie.RootHash()
	require.Equal(t, ledger.GetDefaultHashForHeight(ledger.NodeMaxHeight), hash.Hash(rootHash))

	// verify root hash
	expectedRootHashHex := "568f4ec740fe3b5de88034cb7b1fbddb41548b068f31aebc8ae9189e429c5749"
	require.Equal(t, expectedRootHashHex, hashToString(rootHash))

	// check String() method does not panic:
	_ = emptyTrie.String()
}

// Test_TrieWithLeftRegister tests whether the root hash of trie with only the left-most
// register populated matches the formal specification.
// The expected value is coming from a reference implementation in python and is hard-coded here.
func Test_TrieWithLeftRegister(t *testing.T) {
	// Make new Trie (independently of MForest):
	emptyTrie := trie.NewEmptyMTrie()
	path := utils.PathByUint16LeftPadded(0)
	payload := utils.LightPayload(11, 12345)
	leftPopulatedTrie, maxDepthTouched, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, []ledger.Path{path}, []ledger.Payload{*payload}, true)
	require.NoError(t, err)
	require.Equal(t, uint16(0), maxDepthTouched)
	require.Equal(t, uint64(1), leftPopulatedTrie.AllocatedRegCount())
	require.Equal(t, uint64(payload.Size()), leftPopulatedTrie.AllocatedRegSize())
	expectedRootHashHex := "b30c99cc3e027a6ff463876c638041b1c55316ed935f1b3699e52a2c3e3eaaab"
	require.Equal(t, expectedRootHashHex, hashToString(leftPopulatedTrie.RootHash()))
}

// Test_TrieWithRightRegister tests whether the root hash of trie with only the right-most
// register populated matches the formal specification.
// The expected value is coming from a reference implementation in python and is hard-coded here.
func Test_TrieWithRightRegister(t *testing.T) {
	// Make new Trie (independently of MForest):
	emptyTrie := trie.NewEmptyMTrie()
	// build a path with all 1s
	var path ledger.Path
	for i := 0; i < len(path); i++ {
		path[i] = uint8(255)
	}
	payload := utils.LightPayload(12346, 54321)
	rightPopulatedTrie, maxDepthTouched, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, []ledger.Path{path}, []ledger.Payload{*payload}, true)
	require.NoError(t, err)
	require.Equal(t, uint16(0), maxDepthTouched)
	require.Equal(t, uint64(1), rightPopulatedTrie.AllocatedRegCount())
	require.Equal(t, uint64(payload.Size()), rightPopulatedTrie.AllocatedRegSize())
	expectedRootHashHex := "4313d22bcabbf21b1cfb833d38f1921f06a91e7198a6672bc68fa24eaaa1a961"
	require.Equal(t, expectedRootHashHex, hashToString(rightPopulatedTrie.RootHash()))
}

// Test_TrieWithMiddleRegister tests the root hash of trie holding only a single
// allocated register somewhere in the middle.
// The expected value is coming from a reference implementation in python and is hard-coded here.
func Test_TrieWithMiddleRegister(t *testing.T) {
	// Make new Trie (independently of MForest):
	emptyTrie := trie.NewEmptyMTrie()

	path := utils.PathByUint16LeftPadded(56809)
	payload := utils.LightPayload(12346, 59656)
	leftPopulatedTrie, maxDepthTouched, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, []ledger.Path{path}, []ledger.Payload{*payload}, true)
	require.Equal(t, uint16(0), maxDepthTouched)
	require.Equal(t, uint64(1), leftPopulatedTrie.AllocatedRegCount())
	require.Equal(t, uint64(payload.Size()), leftPopulatedTrie.AllocatedRegSize())
	require.NoError(t, err)
	expectedRootHashHex := "4a29dad0b7ae091a1f035955e0c9aab0692b412f60ae83290b6290d4bf3eb296"
	require.Equal(t, expectedRootHashHex, hashToString(leftPopulatedTrie.RootHash()))
}

// Test_TrieWithManyRegisters tests whether the root hash of a trie storing 12001 randomly selected registers
// matches the formal specification.
// The expected value is coming from a reference implementation in python and is hard-coded here.
func Test_TrieWithManyRegisters(t *testing.T) {
	// Make new Trie (independently of MForest):
	emptyTrie := trie.NewEmptyMTrie()
	// allocate single random register
	rng := &LinearCongruentialGenerator{seed: 0}
	paths, payloads := deduplicateWrites(sampleRandomRegisterWrites(rng, 12001))
	var totalPayloadSize uint64
	for _, p := range payloads {
		totalPayloadSize += uint64(p.Size())
	}
	updatedTrie, maxDepthTouched, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths, payloads, true)
	require.NoError(t, err)
	require.Equal(t, uint16(255), maxDepthTouched)
	require.Equal(t, uint64(12001), updatedTrie.AllocatedRegCount())
	require.Equal(t, totalPayloadSize, updatedTrie.AllocatedRegSize())
	expectedRootHashHex := "74f748dbe563bb5819d6c09a34362a048531fd9647b4b2ea0b6ff43f200198aa"
	require.Equal(t, expectedRootHashHex, hashToString(updatedTrie.RootHash()))
}

// Test_FullTrie tests whether the root hash of a trie,
// whose left-most 65536 registers are populated, matches the formal specification.
// The expected value is coming from a reference implementation in python and is hard-coded here.
func Test_FullTrie(t *testing.T) {
	// Make new Trie (independently of MForest):
	emptyTrie := trie.NewEmptyMTrie()

	// allocate 65536 left-most registers
	numberRegisters := 65536
	rng := &LinearCongruentialGenerator{seed: 0}
	paths := make([]ledger.Path, 0, numberRegisters)
	payloads := make([]ledger.Payload, 0, numberRegisters)
	var totalPayloadSize uint64
	for i := 0; i < numberRegisters; i++ {
		paths = append(paths, utils.PathByUint16LeftPadded(uint16(i)))
		temp := rng.next()
		payload := utils.LightPayload(temp, temp)
		payloads = append(payloads, *payload)
		totalPayloadSize += uint64(payload.Size())
	}
	updatedTrie, maxDepthTouched, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths, payloads, true)
	require.NoError(t, err)
	require.Equal(t, uint16(256), maxDepthTouched)
	require.Equal(t, uint64(numberRegisters), updatedTrie.AllocatedRegCount())
	require.Equal(t, totalPayloadSize, updatedTrie.AllocatedRegSize())
	expectedRootHashHex := "6b3a48d672744f5586c571c47eae32d7a4a3549c1d4fa51a0acfd7b720471de9"
	require.Equal(t, expectedRootHashHex, hashToString(updatedTrie.RootHash()))
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
	emptyTrie := trie.NewEmptyMTrie()

	// allocate single random register
	rng := &LinearCongruentialGenerator{seed: 0}
	path := utils.PathByUint16LeftPadded(rng.next())
	temp := rng.next()
	payload := utils.LightPayload(temp, temp)
	updatedTrie, maxDepthTouched, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, []ledger.Path{path}, []ledger.Payload{*payload}, true)
	require.NoError(t, err)
	require.Equal(t, uint16(0), maxDepthTouched)
	require.Equal(t, uint64(1), updatedTrie.AllocatedRegCount())
	require.Equal(t, uint64(payload.Size()), updatedTrie.AllocatedRegSize())
	expectedRootHashHex := "08db9aeed2b9fcc66b63204a26a4c28652e44e3035bd87ba0ed632a227b3f6dd"
	require.Equal(t, expectedRootHashHex, hashToString(updatedTrie.RootHash()))

	var paths []ledger.Path
	var payloads []ledger.Payload
	parentTrieRegCount := updatedTrie.AllocatedRegCount()
	parentTrieRegSize := updatedTrie.AllocatedRegSize()
	for r := 0; r < 20; r++ {
		paths, payloads = deduplicateWrites(sampleRandomRegisterWrites(rng, r*100))
		var totalPayloadSize uint64
		for _, p := range payloads {
			totalPayloadSize += uint64(p.Size())
		}
		updatedTrie, maxDepthTouched, err = trie.NewTrieWithUpdatedRegisters(updatedTrie, paths, payloads, true)
		require.NoError(t, err)
		switch r {
		case 0:
			require.Equal(t, uint16(0), maxDepthTouched)
		case 1:
			require.Equal(t, uint16(254), maxDepthTouched)
		default:
			require.Equal(t, uint16(255), maxDepthTouched)
		}
		require.Equal(t, parentTrieRegCount+uint64(len(payloads)), updatedTrie.AllocatedRegCount())
		require.Equal(t, parentTrieRegSize+totalPayloadSize, updatedTrie.AllocatedRegSize())
		require.Equal(t, expectedRootHashes[r], hashToString(updatedTrie.RootHash()))

		parentTrieRegCount = updatedTrie.AllocatedRegCount()
		parentTrieRegSize = updatedTrie.AllocatedRegSize()
	}
	// update with the same registers with the same values
	newTrie, maxDepthTouched, err := trie.NewTrieWithUpdatedRegisters(updatedTrie, paths, payloads, true)
	require.NoError(t, err)
	require.Equal(t, uint16(255), maxDepthTouched)
	require.Equal(t, updatedTrie.AllocatedRegCount(), newTrie.AllocatedRegCount())
	require.Equal(t, updatedTrie.AllocatedRegSize(), newTrie.AllocatedRegSize())
	require.Equal(t, expectedRootHashes[19], hashToString(updatedTrie.RootHash()))
	// check the root node pointers are equal
	require.True(t, updatedTrie.RootNode() == newTrie.RootNode())
}

// Test_UnallocateRegisters tests whether unallocating registers matches the formal specification.
// Unallocating here means, to set the stored register value to an empty byte slice.
// The expected value is coming from a reference implementation in python and is hard-coded here.
func Test_UnallocateRegisters(t *testing.T) {
	rng := &LinearCongruentialGenerator{seed: 0}
	emptyTrie := trie.NewEmptyMTrie()

	// we first draw 99 random key-value pairs that will be first allocated and later unallocated:
	paths1, payloads1 := deduplicateWrites(sampleRandomRegisterWrites(rng, 99))
	var totalPayloadSize1 uint64
	for _, p := range payloads1 {
		totalPayloadSize1 += uint64(p.Size())
	}
	updatedTrie, maxDepthTouched, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths1, payloads1, true)
	require.NoError(t, err)
	require.Equal(t, uint16(254), maxDepthTouched)
	require.Equal(t, uint64(len(payloads1)), updatedTrie.AllocatedRegCount())
	require.Equal(t, totalPayloadSize1, updatedTrie.AllocatedRegSize())

	// we then write an additional 117 registers
	paths2, payloads2 := deduplicateWrites(sampleRandomRegisterWrites(rng, 117))
	var totalPayloadSize2 uint64
	for _, p := range payloads2 {
		totalPayloadSize2 += uint64(p.Size())
	}
	updatedTrie, maxDepthTouched, err = trie.NewTrieWithUpdatedRegisters(updatedTrie, paths2, payloads2, true)
	require.Equal(t, uint16(254), maxDepthTouched)
	require.Equal(t, uint64(len(payloads1)+len(payloads2)), updatedTrie.AllocatedRegCount())
	require.Equal(t, totalPayloadSize1+totalPayloadSize2, updatedTrie.AllocatedRegSize())
	require.NoError(t, err)

	// and now we override the first 99 registers with default values, i.e. unallocate them
	payloads0 := make([]ledger.Payload, len(payloads1))
	updatedTrie, maxDepthTouched, err = trie.NewTrieWithUpdatedRegisters(updatedTrie, paths1, payloads0, true)
	require.Equal(t, uint16(254), maxDepthTouched)
	require.Equal(t, uint64(len(payloads2)), updatedTrie.AllocatedRegCount())
	require.Equal(t, totalPayloadSize2, updatedTrie.AllocatedRegSize())
	require.NoError(t, err)

	// this should be identical to the first 99 registers never been written
	expectedRootHashHex := "d81e27a93f2bef058395f70e00fb5d3c8e426e22b3391d048b34017e1ecb483e"
	comparisonTrie, maxDepthTouched, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths2, payloads2, true)
	require.NoError(t, err)
	require.Equal(t, uint16(254), maxDepthTouched)
	require.Equal(t, uint64(len(payloads2)), comparisonTrie.AllocatedRegCount())
	require.Equal(t, totalPayloadSize2, comparisonTrie.AllocatedRegSize())
	require.Equal(t, expectedRootHashHex, hashToString(comparisonTrie.RootHash()))
	require.Equal(t, expectedRootHashHex, hashToString(updatedTrie.RootHash()))
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

// sampleRandomRegisterWritesWithPrefix generates path-payload tuples for `number` randomly selected registers;
// each path is starting with the specified `prefix` and is filled to the full length with random bytes
// caution: register paths might repeat
func sampleRandomRegisterWritesWithPrefix(rng *LinearCongruentialGenerator, number int, prefix []byte) ([]ledger.Path, []ledger.Payload) {
	prefixLen := len(prefix)
	if prefixLen >= hash.HashLen {
		panic("prefix must be shorter than full path length, so there is some space left for random path segment")
	}

	paths := make([]ledger.Path, 0, number)
	payloads := make([]ledger.Payload, 0, number)
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
		payload := utils.LightPayload(t, t)
		payloads = append(payloads, *payload)
	}
	return paths, payloads
}

// deduplicateWrites retains only the last register write
func deduplicateWrites(paths []ledger.Path, payloads []ledger.Payload) ([]ledger.Path, []ledger.Payload) {
	payloadMapping := make(map[ledger.Path]int)
	if len(paths) != len(payloads) {
		panic("size mismatch (paths and payloads)")
	}
	for i, path := range paths {
		// we override the latest in the slice
		payloadMapping[path] = i
	}
	dedupedPaths := make([]ledger.Path, 0, len(payloadMapping))
	dedupedPayloads := make([]ledger.Payload, 0, len(payloadMapping))
	for path := range payloadMapping {
		dedupedPaths = append(dedupedPaths, path)
		dedupedPayloads = append(dedupedPayloads, payloads[payloadMapping[path]])
	}
	return dedupedPaths, dedupedPayloads
}

func TestSplitByPath(t *testing.T) {
	seed := time.Now().UnixNano()
	t.Logf("rand seed is %d", seed)
	rand.Seed(seed)

	const pathsNumber = 100
	const redundantPaths = 10
	const pathsSize = 32
	randomIndex := rand.Intn(pathsSize)

	// create path slice with redundant paths
	paths := make([]ledger.Path, 0, pathsNumber)
	for i := 0; i < pathsNumber-redundantPaths; i++ {
		var p ledger.Path
		rand.Read(p[:])
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
	index := trie.SplitPaths(paths, randomIndex)

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
//  * By convention, a node in the trie is a leaf if both children are nil.
//  * Therefore, we consider a completely unallocated subtrie also as a potential leaf.
// An edge case can now arise when unallocating a previously allocated leaf (see vertex '■' in the illustration below):
//  * Before the update, both children of the leaf are nil (because it is a leaf)
//  * After the update-algorithm updated the sub-Trie with root ■, both children of the updated vertex are
//    also nil. But the sub-trie has now changed: the register previously represented by ■ is now gone.
//    This case must be explicitly handled by the update algorithm:
//    (i)  If the vertex is an interim node, i.e. it had at least one child, it is legal to re-use the vertex if neither
//         of its child-subtries were affected by the update.
//    (ii) If the vertex is a leaf, only checking that neither child-subtries were affected by the update is insufficient.
//         This is because the register the leaf represents might itself be affected by the update.
//    Condition (ii) is particularly subtle, if there are register updates in the subtrie of the leaf:
//     * From an API perspective, it is a legal operation to set an unallocated register to nil (essentially a no-op).
//     * Though, the Trie-update algorithm only realizes that the register is already unallocated, once it traverses
//       into the respective sub-trie. When bubbling up from the recursion, nothing has changed in the children of ■
//       but the vertex ■ itself has changed from an allocated leaf register to an unallocated register.
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
	leftSubTriePath, leftSubTriePayload := sampleRandomRegisterWritesWithPrefix(rng, 1, leftSubTriePrefix)
	rightSubTriePath, rightSubTriePayload := deduplicateWrites(sampleRandomRegisterWritesWithPrefix(rng, 18, rightSubTriePrefix))

	// initialize Trie to the depicted state
	paths := append(leftSubTriePath, rightSubTriePath...)
	payloads := append(leftSubTriePayload, rightSubTriePayload...)
	var leftSubTriePayloadSize, rightSubTriePayloadSize uint64
	for _, p := range leftSubTriePayload {
		leftSubTriePayloadSize += uint64(p.Size())
	}
	for _, p := range rightSubTriePayload {
		rightSubTriePayloadSize += uint64(p.Size())
	}
	startTrie, maxDepthTouched, err := trie.NewTrieWithUpdatedRegisters(trie.NewEmptyMTrie(), paths, payloads, true)
	require.NoError(t, err)
	require.Equal(t, uint16(241), maxDepthTouched)
	require.Equal(t, uint64(len(payloads)), startTrie.AllocatedRegCount())
	require.Equal(t, leftSubTriePayloadSize+rightSubTriePayloadSize, startTrie.AllocatedRegSize())
	expectedRootHashHex := "8cf6659db0af7626ab0991e2a49019353d549aa4a8c4be1b33e8953d1a9b7fdd"
	require.Equal(t, expectedRootHashHex, hashToString(startTrie.RootHash()))

	// Register update:
	//  * de-allocate the compactified leaf (■), i.e. set its payload to nil.
	//  * also set a previously already unallocated register to nil as well
	unallocatedRegister := leftSubTriePath[0]            // copy path to leaf and modify it (next line)
	unallocatedRegister[len(unallocatedRegister)-1] ^= 1 // path differs only in the last byte, i.e. register is also in the left Sub-Trie
	updatedPaths := append(leftSubTriePath, unallocatedRegister)
	updatedPayloads := []ledger.Payload{*ledger.EmptyPayload(), *ledger.EmptyPayload()}
	updatedTrie, maxDepthTouched, err := trie.NewTrieWithUpdatedRegisters(startTrie, updatedPaths, updatedPayloads, true)
	require.Equal(t, uint16(256), maxDepthTouched)
	require.Equal(t, uint64(len(rightSubTriePayload)), updatedTrie.AllocatedRegCount())
	require.Equal(t, rightSubTriePayloadSize, updatedTrie.AllocatedRegSize())
	require.NoError(t, err)

	// The updated trie should equal to a trie containing only the right sub-Trie
	expectedUpdatedRootHashHex := "576e12a7ef5c760d5cc808ce50e9297919b21b87656b0cc0d9fe8a1a589cf42c"
	require.Equal(t, expectedUpdatedRootHashHex, hashToString(updatedTrie.RootHash()))
	referenceTrie, maxDepthTouched, err := trie.NewTrieWithUpdatedRegisters(trie.NewEmptyMTrie(), rightSubTriePath, rightSubTriePayload, true)
	require.NoError(t, err)
	require.Equal(t, uint16(241), maxDepthTouched)
	require.Equal(t, uint64(len(rightSubTriePayload)), referenceTrie.AllocatedRegCount())
	require.Equal(t, rightSubTriePayloadSize, referenceTrie.AllocatedRegSize())
	require.Equal(t, expectedUpdatedRootHashHex, hashToString(referenceTrie.RootHash()))
}

func Test_Pruning(t *testing.T) {
	emptyTrie := trie.NewEmptyMTrie()

	path1 := utils.PathByUint16(1 << 12)       // 000100...
	path2 := utils.PathByUint16(1 << 13)       // 001000...
	path4 := utils.PathByUint16(1<<14 + 1<<13) // 01100...
	path6 := utils.PathByUint16(1 << 15)       // 1000...

	payload1 := utils.LightPayload(2, 1)
	payload2 := utils.LightPayload(2, 2)
	payload4 := utils.LightPayload(2, 4)
	payload6 := utils.LightPayload(2, 6)
	emptyPayload := ledger.EmptyPayload()

	paths := []ledger.Path{path1, path2, path4, path6}
	payloads := []ledger.Payload{*payload1, *payload2, *payload4, *payload6}

	var totalPayloadSize uint64
	for _, p := range payloads {
		totalPayloadSize += uint64(p.Size())
	}

	//                    n7
	//                   / \
	//                 /     \
	//             n5         n6 (path6/payload6) // 1000
	//            /  \
	//          /      \
	//         /         \
	//        n3          n4 (path4/payload4) // 01100...
	//      /     \
	//    /          \
	//  /              \
	// n1 (path1,       n2 (path2)
	//     payload1)        /payload2)

	baseTrie, maxDepthTouched, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths, payloads, true)
	require.NoError(t, err)
	require.Equal(t, uint16(3), maxDepthTouched)
	require.Equal(t, uint64(len(payloads)), baseTrie.AllocatedRegCount())
	require.Equal(t, totalPayloadSize, baseTrie.AllocatedRegSize())

	t.Run("leaf update with pruning test", func(t *testing.T) {
		expectedRegCount := baseTrie.AllocatedRegCount() - 1
		expectedRegSize := baseTrie.AllocatedRegSize() - uint64(payload1.Size())

		trie1, maxDepthTouched, err := trie.NewTrieWithUpdatedRegisters(baseTrie, []ledger.Path{path1}, []ledger.Payload{*emptyPayload}, false)
		require.NoError(t, err)
		require.Equal(t, uint16(3), maxDepthTouched)
		require.Equal(t, expectedRegCount, trie1.AllocatedRegCount())
		require.Equal(t, expectedRegSize, trie1.AllocatedRegSize())

		trie1withpruning, maxDepthTouched, err := trie.NewTrieWithUpdatedRegisters(baseTrie, []ledger.Path{path1}, []ledger.Payload{*emptyPayload}, true)
		require.NoError(t, err)
		require.Equal(t, uint16(3), maxDepthTouched)
		require.Equal(t, expectedRegCount, trie1withpruning.AllocatedRegCount())
		require.Equal(t, expectedRegSize, trie1withpruning.AllocatedRegSize())
		require.True(t, trie1withpruning.IsAValidTrie())

		// after pruning
		//                    n7
		//                   / \
		//                 /     \
		//             n5         n6 (path6/payload6) // 1000
		//            /  \
		//          /      \
		//         /         \
		//     n3 (path2       n4 (path4
		//        /payload2)      /payload4) // 01100...
		require.Equal(t, trie1.RootHash(), trie1withpruning.RootHash())
	})

	t.Run("leaf update with two level pruning test", func(t *testing.T) {
		expectedRegCount := baseTrie.AllocatedRegCount() - 1
		expectedRegSize := baseTrie.AllocatedRegSize() - uint64(payload4.Size())

		// setting path4 to zero from baseTrie
		trie2, maxDepthTouched, err := trie.NewTrieWithUpdatedRegisters(baseTrie, []ledger.Path{path4}, []ledger.Payload{*emptyPayload}, false)
		require.NoError(t, err)
		require.Equal(t, uint16(2), maxDepthTouched)
		require.Equal(t, expectedRegCount, trie2.AllocatedRegCount())
		require.Equal(t, expectedRegSize, trie2.AllocatedRegSize())

		// pruning is not activated here because n3 is not a leaf node
		trie2withpruning, maxDepthTouched, err := trie.NewTrieWithUpdatedRegisters(baseTrie, []ledger.Path{path4}, []ledger.Payload{*emptyPayload}, true)
		require.NoError(t, err)
		require.Equal(t, uint16(2), maxDepthTouched)
		require.Equal(t, expectedRegCount, trie2withpruning.AllocatedRegCount())
		require.Equal(t, expectedRegSize, trie2withpruning.AllocatedRegSize())
		require.True(t, trie2withpruning.IsAValidTrie())

		require.Equal(t, trie2.RootHash(), trie2withpruning.RootHash())

		// now setting path2 to zero should do the pruning for two levels
		expectedRegCount -= 1
		expectedRegSize -= uint64(payload2.Size())

		trie22, maxDepthTouched, err := trie.NewTrieWithUpdatedRegisters(trie2, []ledger.Path{path2}, []ledger.Payload{*emptyPayload}, false)
		require.NoError(t, err)
		require.Equal(t, uint16(3), maxDepthTouched)
		require.Equal(t, expectedRegCount, trie22.AllocatedRegCount())
		require.Equal(t, expectedRegSize, trie22.AllocatedRegSize())

		trie22withpruning, maxDepthTouched, err := trie.NewTrieWithUpdatedRegisters(trie2withpruning, []ledger.Path{path2}, []ledger.Payload{*emptyPayload}, true)
		require.NoError(t, err)
		require.Equal(t, uint16(3), maxDepthTouched)
		require.Equal(t, expectedRegCount, trie22withpruning.AllocatedRegCount())
		require.Equal(t, expectedRegSize, trie22withpruning.AllocatedRegSize())

		// after pruning
		//                     n7
		//                   /   \
		//                 /       \
		//             n5 (path1,   n6 (path6/payload6) // 1000
		//                 /payload1)

		require.Equal(t, trie22.RootHash(), trie22withpruning.RootHash())
		require.True(t, trie22withpruning.IsAValidTrie())

	})

	t.Run("several updates at the same time", func(t *testing.T) {
		// setting path4 to zero from baseTrie
		trie3, maxDepthTouched, err := trie.NewTrieWithUpdatedRegisters(baseTrie, []ledger.Path{path2, path4, path6}, []ledger.Payload{*emptyPayload, *emptyPayload, *emptyPayload}, false)
		require.NoError(t, err)
		require.Equal(t, uint16(3), maxDepthTouched)
		require.Equal(t, uint64(1), trie3.AllocatedRegCount())
		require.Equal(t, uint64(payload1.Size()), trie3.AllocatedRegSize())

		// this should prune two levels
		trie3withpruning, maxDepthTouched, err := trie.NewTrieWithUpdatedRegisters(baseTrie, []ledger.Path{path2, path4, path6}, []ledger.Payload{*emptyPayload, *emptyPayload, *emptyPayload}, true)
		require.NoError(t, err)
		require.Equal(t, uint16(3), maxDepthTouched)
		require.Equal(t, uint64(1), trie3withpruning.AllocatedRegCount())
		require.Equal(t, uint64(payload1.Size()), trie3withpruning.AllocatedRegSize())

		// after pruning
		//       n7  (path1/payload1)
		require.Equal(t, trie3.RootHash(), trie3withpruning.RootHash())
		require.True(t, trie3withpruning.IsAValidTrie())
	})

	t.Run("smoke testing trie pruning", func(t *testing.T) {
		unittest.SkipUnless(t, unittest.TEST_LONG_RUNNING, "skipping trie pruning smoke testing as its not needed to always run")

		numberOfSteps := 1000
		numberOfUpdates := 750
		numberOfRemovals := 750

		var err error
		activeTrie := trie.NewEmptyMTrie()
		activeTrieWithPruning := trie.NewEmptyMTrie()
		allPaths := make(map[ledger.Path]ledger.Payload)
		var maxDepthTouched, maxDepthTouchedWithPruning uint16
		var parentTrieRegCount, parentTrieRegSize uint64

		for step := 0; step < numberOfSteps; step++ {

			updatePaths := make([]ledger.Path, 0)
			updatePayloads := make([]ledger.Payload, 0)

			var expectedRegCountDelta int64
			var expectedRegSizeDelta int64

			for i := 0; i < numberOfUpdates; {
				var path ledger.Path
				rand.Read(path[:])
				// deduplicate
				if _, found := allPaths[path]; !found {
					payload := utils.RandomPayload(1, 100)
					updatePaths = append(updatePaths, path)
					updatePayloads = append(updatePayloads, *payload)
					expectedRegCountDelta++
					expectedRegSizeDelta += int64(payload.Size())
					i++
				}
			}

			i := 0
			samplesNeeded := int(math.Min(float64(numberOfRemovals), float64(len(allPaths))))
			for p, pl := range allPaths {
				updatePaths = append(updatePaths, p)
				updatePayloads = append(updatePayloads, *emptyPayload)
				expectedRegCountDelta--
				expectedRegSizeDelta -= int64(pl.Size())
				delete(allPaths, p)
				i++
				if i > samplesNeeded {
					break
				}
			}

			// only set it for the updates
			for i := 0; i < numberOfUpdates; i++ {
				allPaths[updatePaths[i]] = updatePayloads[i]
			}

			activeTrie, maxDepthTouched, err = trie.NewTrieWithUpdatedRegisters(activeTrie, updatePaths, updatePayloads, false)
			require.NoError(t, err)
			require.Equal(t, uint64(int64(parentTrieRegCount)+expectedRegCountDelta), activeTrie.AllocatedRegCount())
			require.Equal(t, uint64(int64(parentTrieRegSize)+expectedRegSizeDelta), activeTrie.AllocatedRegSize())

			activeTrieWithPruning, maxDepthTouchedWithPruning, err = trie.NewTrieWithUpdatedRegisters(activeTrieWithPruning, updatePaths, updatePayloads, true)
			require.NoError(t, err)
			require.True(t, maxDepthTouched >= maxDepthTouchedWithPruning)
			require.Equal(t, uint64(int64(parentTrieRegCount)+expectedRegCountDelta), activeTrieWithPruning.AllocatedRegCount())
			require.Equal(t, uint64(int64(parentTrieRegSize)+expectedRegSizeDelta), activeTrieWithPruning.AllocatedRegSize())

			require.Equal(t, activeTrie.RootHash(), activeTrieWithPruning.RootHash())

			parentTrieRegCount = activeTrie.AllocatedRegCount()
			parentTrieRegSize = activeTrie.AllocatedRegSize()

			// fetch all values and compare
			queryPaths := make([]ledger.Path, 0)
			for path := range allPaths {
				queryPaths = append(queryPaths, path)
			}

			payloads := activeTrie.UnsafeRead(queryPaths)
			for i, pp := range payloads {
				expectedPayload := allPaths[queryPaths[i]]
				require.True(t, pp.Equals(&expectedPayload))
			}

			payloads = activeTrieWithPruning.UnsafeRead(queryPaths)
			for i, pp := range payloads {
				expectedPayload := allPaths[queryPaths[i]]
				require.True(t, pp.Equals(&expectedPayload))
			}

		}
	})
}

func hashToString(hash ledger.RootHash) string {
	return hex.EncodeToString(hash[:])
}

// TestValueSizes tests value sizes of existent and non-existent paths for trie of different layouts.
func TestValueSizes(t *testing.T) {

	emptyTrie := trie.NewEmptyMTrie()

	// Test value sizes for non-existent path in empty trie
	t.Run("empty trie", func(t *testing.T) {
		path := utils.PathByUint16LeftPadded(0)
		pathsToGetValueSize := []ledger.Path{path}
		sizes := emptyTrie.UnsafeValueSizes(pathsToGetValueSize)
		require.Equal(t, len(pathsToGetValueSize), len(sizes))
		require.Equal(t, 0, sizes[0])
	})

	// Test value sizes for a mix of existent and non-existent paths
	// in trie with compact leaf as root node.
	t.Run("compact leaf as root", func(t *testing.T) {
		path1 := utils.PathByUint16LeftPadded(0)
		payload1 := utils.RandomPayload(1, 100)

		path2 := utils.PathByUint16LeftPadded(1) // This path will not be inserted into trie.

		paths := []ledger.Path{path1}
		payloads := []ledger.Payload{*payload1}

		newTrie, maxDepthTouched, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths, payloads, true)
		require.NoError(t, err)
		require.Equal(t, uint16(0), maxDepthTouched)

		pathsToGetValueSize := []ledger.Path{path1, path2}

		sizes := newTrie.UnsafeValueSizes(pathsToGetValueSize)
		require.Equal(t, len(pathsToGetValueSize), len(sizes))
		require.Equal(t, payload1.Value.Size(), sizes[0])
		require.Equal(t, 0, sizes[1])
	})

	// Test value sizes for a mix of existent and non-existent paths in partial trie.
	t.Run("partial trie", func(t *testing.T) {
		path1 := utils.PathByUint16(1 << 12) // 000100...
		path2 := utils.PathByUint16(1 << 13) // 001000...

		payload1 := utils.RandomPayload(1, 100)
		payload2 := utils.RandomPayload(1, 100)

		paths := []ledger.Path{path1, path2}
		payloads := []ledger.Payload{*payload1, *payload2}

		// Create a new trie with 2 leaf nodes (n1 and n2) at height 253.
		newTrie, maxDepthTouched, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths, payloads, true)
		require.NoError(t, err)
		require.Equal(t, uint16(3), maxDepthTouched)

		//                  n5
		//                 /
		//                /
		//              n4
		//             /
		//            /
		//           n3
		//        /     \
		//      /         \
		//   n1 (path1/     n2 (path2/
		//       payload1)      payload2)
		//

		// Populate pathsToGetValueSize with all possible paths for the first 4 bits.
		pathsToGetValueSize := make([]ledger.Path, 16)
		for i := 0; i < 16; i++ {
			pathsToGetValueSize[i] = utils.PathByUint16(uint16(i << 12))
		}

		// Test value sizes for a mix of existent and non-existent paths.
		sizes := newTrie.UnsafeValueSizes(pathsToGetValueSize)
		require.Equal(t, len(pathsToGetValueSize), len(sizes))
		for i, p := range pathsToGetValueSize {
			switch p {
			case path1:
				require.Equal(t, payload1.Value.Size(), sizes[i])
			case path2:
				require.Equal(t, payload2.Value.Size(), sizes[i])
			default:
				// Test value size for non-existent path
				require.Equal(t, 0, sizes[i])
			}
		}

		// Test value size for a single existent path
		pathsToGetValueSize = []ledger.Path{path1}
		sizes = newTrie.UnsafeValueSizes(pathsToGetValueSize)
		require.Equal(t, len(pathsToGetValueSize), len(sizes))
		require.Equal(t, payload1.Value.Size(), sizes[0])

		// Test value size for a single non-existent path
		pathsToGetValueSize = []ledger.Path{utils.PathByUint16(3 << 12)}
		sizes = newTrie.UnsafeValueSizes(pathsToGetValueSize)
		require.Equal(t, len(pathsToGetValueSize), len(sizes))
		require.Equal(t, 0, sizes[0])
	})
}

// TestValueSizesWithDuplicatePaths tests value sizes of duplicate existent and non-existent paths.
func TestValueSizesWithDuplicatePaths(t *testing.T) {
	path1 := utils.PathByUint16(0)
	path2 := utils.PathByUint16(1)
	path3 := utils.PathByUint16(2) // This path will not be inserted into trie.

	payload1 := utils.RandomPayload(1, 100)
	payload2 := utils.RandomPayload(1, 100)

	paths := []ledger.Path{path1, path2}
	payloads := []ledger.Payload{*payload1, *payload2}

	emptyTrie := trie.NewEmptyMTrie()
	newTrie, maxDepthTouched, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths, payloads, true)
	require.NoError(t, err)
	require.Equal(t, uint16(16), maxDepthTouched)

	// pathsToGetValueSize is a mix of duplicate existent and nonexistent paths.
	pathsToGetValueSize := []ledger.Path{
		path1, path2, path3,
		path1, path2, path3,
	}

	sizes := newTrie.UnsafeValueSizes(pathsToGetValueSize)
	require.Equal(t, len(pathsToGetValueSize), len(sizes))
	for i, p := range pathsToGetValueSize {
		switch p {
		case path1:
			require.Equal(t, payload1.Value.Size(), sizes[i])
		case path2:
			require.Equal(t, payload2.Value.Size(), sizes[i])
		default:
			// Test payload size for non-existent path
			require.Equal(t, 0, sizes[i])
		}
	}
}

// TestTrieAllocatedRegCountRegSize tests allocated register count and register size for updated trie.
// It tests the following updates with prune flag set to true:
//   * update empty trie with new paths and payloads
//   * update trie with existing paths and updated payload
//   * update trie with new paths and empty payloads
//   * update trie with existing path and empty payload one by one until trie is empty
// It also tests the following updates with prune flag set to false:
//   * update trie with existing path and empty payload one by one until trie is empty
//   * update trie with removed paths and empty payloads
//   * update trie with removed paths and non-empty payloads
func TestTrieAllocatedRegCountRegSize(t *testing.T) {

	rng := &LinearCongruentialGenerator{seed: 0}

	// Allocate 255 registers
	numberRegisters := 255
	paths := make([]ledger.Path, numberRegisters)
	payloads := make([]ledger.Payload, numberRegisters)
	var totalPayloadSize uint64
	for i := 0; i < numberRegisters; i++ {
		var p ledger.Path
		p[0] = byte(i)

		payload := utils.LightPayload(rng.next(), rng.next())
		paths[i] = p
		payloads[i] = *payload

		totalPayloadSize += uint64(payload.Size())
	}

	// Update trie with registers to test reg count and size with new registers.
	updatedTrie, maxDepthTouched, err := trie.NewTrieWithUpdatedRegisters(trie.NewEmptyMTrie(), paths, payloads, true)
	require.NoError(t, err)
	require.True(t, maxDepthTouched <= 256)
	require.Equal(t, uint64(len(payloads)), updatedTrie.AllocatedRegCount())
	require.Equal(t, totalPayloadSize, updatedTrie.AllocatedRegSize())

	// Update trie with existing paths and updated payloads
	// to test reg count and size with updated registers
	// (old payload size > 0 and new payload size > 0).
	for i := 0; i < len(payloads); i += 2 {
		newPayload := utils.LightPayload(rng.next(), rng.next())
		oldPayload := payloads[i]
		payloads[i] = *newPayload
		totalPayloadSize += uint64(newPayload.Size()) - uint64(oldPayload.Size())
	}

	updatedTrie, maxDepthTouched, err = trie.NewTrieWithUpdatedRegisters(updatedTrie, paths, payloads, true)
	require.NoError(t, err)
	require.True(t, maxDepthTouched <= 256)
	require.Equal(t, uint64(len(payloads)), updatedTrie.AllocatedRegCount())
	require.Equal(t, totalPayloadSize, updatedTrie.AllocatedRegSize())

	rootHash := updatedTrie.RootHash()

	// Update trie with new paths and empty payloads
	// to test reg count and size with new empty registers.
	newPaths := []ledger.Path{}
	newPayloads := []ledger.Payload{}
	for i := 0; i < len(paths); i++ {
		oldPath := paths[i]

		path1, _ := ledger.ToPath(oldPath[:])
		path1[1] = 1
		payload1 := ledger.Payload{Key: ledger.Key{KeyParts: []ledger.KeyPart{{Type: 0, Value: []byte{0x00, byte(i)}}}}}

		path2, _ := ledger.ToPath(oldPath[:])
		path2[1] = 2
		payload2 := ledger.EmptyPayload()

		newPaths = append(newPaths, oldPath, path1, path2)
		newPayloads = append(newPayloads, payloads[i], payload1, *payload2)
	}

	updatedTrie, maxDepthTouched, err = trie.NewTrieWithUpdatedRegisters(updatedTrie, newPaths, newPayloads, true)
	require.NoError(t, err)
	require.Equal(t, rootHash, updatedTrie.RootHash())
	require.True(t, maxDepthTouched <= 256)
	require.Equal(t, uint64(len(payloads)), updatedTrie.AllocatedRegCount())
	require.Equal(t, totalPayloadSize, updatedTrie.AllocatedRegSize())

	t.Run("pruning", func(t *testing.T) {
		expectedRegCount := uint64(len(payloads))
		expectedRegSize := totalPayloadSize

		updatedTrieWithPruning := updatedTrie

		// Remove register one by one to test reg count and size with empty registers
		// (old payload size > 0 and new payload size == 0)
		for i := 0; i < len(paths); i++ {
			newPaths := []ledger.Path{paths[i]}
			newPayloads := []ledger.Payload{*ledger.EmptyPayload()}

			expectedRegCount--
			expectedRegSize -= uint64(payloads[i].Size())

			updatedTrieWithPruning, maxDepthTouched, err = trie.NewTrieWithUpdatedRegisters(updatedTrieWithPruning, newPaths, newPayloads, true)
			require.NoError(t, err)
			require.True(t, maxDepthTouched <= 256)
			require.Equal(t, expectedRegCount, updatedTrieWithPruning.AllocatedRegCount())
			require.Equal(t, expectedRegSize, updatedTrieWithPruning.AllocatedRegSize())
		}

		// After all registered are removed, reg count and size should be 0.
		require.Equal(t, trie.EmptyTrieRootHash(), updatedTrieWithPruning.RootHash())
		require.Equal(t, uint64(0), updatedTrieWithPruning.AllocatedRegSize())
		require.Equal(t, uint64(0), updatedTrieWithPruning.AllocatedRegSize())
	})

	t.Run("no pruning", func(t *testing.T) {
		expectedRegCount := uint64(len(payloads))
		expectedRegSize := totalPayloadSize

		updatedTrieNoPruning := updatedTrie

		// Remove register one by one to test reg count and size with empty registers
		// (old payload size > 0 and new payload size == 0)
		for i := 0; i < len(paths); i++ {
			newPaths := []ledger.Path{paths[i]}
			newPayloads := []ledger.Payload{*ledger.EmptyPayload()}

			expectedRegCount--
			expectedRegSize -= uint64(payloads[i].Size())

			updatedTrieNoPruning, maxDepthTouched, err = trie.NewTrieWithUpdatedRegisters(updatedTrieNoPruning, newPaths, newPayloads, false)
			require.NoError(t, err)
			require.True(t, maxDepthTouched <= 256)
			require.Equal(t, expectedRegCount, updatedTrieNoPruning.AllocatedRegCount())
			require.Equal(t, expectedRegSize, updatedTrieNoPruning.AllocatedRegSize())
		}

		// After all registered are removed, reg count and size should be 0.
		require.Equal(t, trie.EmptyTrieRootHash(), updatedTrieNoPruning.RootHash())
		require.Equal(t, uint64(0), updatedTrieNoPruning.AllocatedRegCount())
		require.Equal(t, uint64(0), updatedTrieNoPruning.AllocatedRegSize())

		// Update with removed paths and empty payloads
		// (old payload size == 0 and new payload size == 0)
		newPayloads := make([]ledger.Payload, len(paths))
		for i := 0; i < len(paths); i++ {
			newPayloads[i] = *ledger.EmptyPayload()
		}

		updatedTrieNoPruning, maxDepthTouched, err = trie.NewTrieWithUpdatedRegisters(updatedTrieNoPruning, paths, newPayloads, false)
		require.NoError(t, err)
		require.True(t, maxDepthTouched <= 256)
		require.Equal(t, trie.EmptyTrieRootHash(), updatedTrieNoPruning.RootHash())
		require.Equal(t, uint64(0), updatedTrieNoPruning.AllocatedRegCount())
		require.Equal(t, uint64(0), updatedTrieNoPruning.AllocatedRegSize())

		// Update with removed paths and non-empty payloads
		// (old payload size == 0 and new payload size > 0)
		updatedTrieNoPruning, maxDepthTouched, err = trie.NewTrieWithUpdatedRegisters(updatedTrieNoPruning, paths, payloads, false)
		require.NoError(t, err)
		require.Equal(t, rootHash, updatedTrie.RootHash())
		require.True(t, maxDepthTouched <= 256)
		require.Equal(t, uint64(len(payloads)), updatedTrieNoPruning.AllocatedRegCount())
		require.Equal(t, totalPayloadSize, updatedTrieNoPruning.AllocatedRegSize())
	})
}

// TestTrieAllocatedRegCountRegSizeWithMixedPruneFlag tests allocated register count and size
// for updated trie with mixed pruning flag.
// It tests the following updates:
//   * step 1 : update empty trie with new paths and payloads (255 allocated registers)
//   * step 2 : remove a payload without pruning (254 allocated registers)
//   * step 3a: remove previously removed payload with pruning (254 allocated registers)
//   * step 3b: update trie from step 2 with a new payload (sibling of removed payload)
//              with pruning (255 allocated registers)
func TestTrieAllocatedRegCountRegSizeWithMixedPruneFlag(t *testing.T) {
	rng := &LinearCongruentialGenerator{seed: 0}

	// Allocate 255 registers
	numberRegisters := 255
	paths := make([]ledger.Path, numberRegisters)
	payloads := make([]ledger.Payload, numberRegisters)
	var totalPayloadSize uint64
	for i := 0; i < numberRegisters; i++ {
		var p ledger.Path
		p[0] = byte(i)

		payload := utils.LightPayload(rng.next(), rng.next())
		paths[i] = p
		payloads[i] = *payload

		totalPayloadSize += uint64(payload.Size())
	}

	expectedAllocatedRegCount := uint64(len(payloads))
	expectedAllocatedRegSize := totalPayloadSize

	// Update trie with registers to test reg count and size with new registers.
	baseTrie, maxDepthTouched, err := trie.NewTrieWithUpdatedRegisters(trie.NewEmptyMTrie(), paths, payloads, true)
	require.NoError(t, err)
	require.True(t, maxDepthTouched <= 256)
	require.Equal(t, expectedAllocatedRegCount, baseTrie.AllocatedRegCount())
	require.Equal(t, expectedAllocatedRegSize, baseTrie.AllocatedRegSize())

	// Remove one payload without pruning
	expectedAllocatedRegCount--
	expectedAllocatedRegSize -= uint64(payloads[0].Size())

	removePaths := []ledger.Path{paths[0]}
	removePayloads := []ledger.Payload{*ledger.EmptyPayload()}
	unprunedTrie, maxDepthTouched, err := trie.NewTrieWithUpdatedRegisters(baseTrie, removePaths, removePayloads, false)
	require.NoError(t, err)
	require.True(t, maxDepthTouched <= 256)
	require.Equal(t, expectedAllocatedRegCount, unprunedTrie.AllocatedRegCount())
	require.Equal(t, expectedAllocatedRegSize, unprunedTrie.AllocatedRegSize())

	// Remove the same payload (no affect) from unprunedTrie with pruning
	// expected reg count and reg size remain unchanged.
	updatedTrie, maxDepthTouched, err := trie.NewTrieWithUpdatedRegisters(unprunedTrie, removePaths, removePayloads, true)
	require.NoError(t, err)
	require.True(t, maxDepthTouched <= 256)
	require.Equal(t, expectedAllocatedRegCount, updatedTrie.AllocatedRegCount())
	require.Equal(t, expectedAllocatedRegSize, updatedTrie.AllocatedRegSize())

	// Add sibling of removed path from unprunedTrie with pruning
	newPath := paths[0]
	bitutils.SetBit(newPath[:], ledger.PathLen*8-1)
	newPaths := []ledger.Path{newPath}
	newPayloads := []ledger.Payload{*utils.LightPayload(rng.next(), rng.next())}

	// expected reg count is incremented and expected reg size is increase by new payload size.
	expectedAllocatedRegCount++
	expectedAllocatedRegSize += uint64(newPayloads[0].Size())

	updatedTrie, maxDepthTouched, err = trie.NewTrieWithUpdatedRegisters(unprunedTrie, newPaths, newPayloads, true)
	require.NoError(t, err)
	require.True(t, maxDepthTouched <= 256)
	require.Equal(t, expectedAllocatedRegCount, updatedTrie.AllocatedRegCount())
	require.Equal(t, expectedAllocatedRegSize, updatedTrie.AllocatedRegSize())
}
