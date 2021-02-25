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
	// ReferenceImplPathByteSize is the path length in reference implementation: 2 bytes.
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

	path := utils.PathByUint16(0)
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

	path := utils.PathByUint16(65535)
	payload := utils.LightPayload(12346, 54321)
	rightPopulatedTrie, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, []ledger.Path{path}, []ledger.Payload{*payload})
	require.NoError(t, err)
	expectedRootHashHex := "d9ddb92fc7471650cf97002c8115177fa4cee420e447f10c2dd2ac8c6fe6643c"
	require.Equal(t, expectedRootHashHex, hex.EncodeToString(rightPopulatedTrie.RootHash()))
}

// // Test_TrieWithMiddleRegister tests the root hash of trie holding only a single
// // allocated register somewhere in the middle.
// // The expected value is coming from a reference implementation in python and is hard-coded here.
func Test_TrieWithMiddleRegister(t *testing.T) {
	// Make new Trie (independently of MForest):
	emptyTrie, err := trie.NewEmptyMTrie(ReferenceImplPathByteSize)
	require.NoError(t, err)

	path := utils.PathByUint16(56809)
	payload := utils.LightPayload(12346, 59656)
	leftPopulatedTrie, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, []ledger.Path{path}, []ledger.Payload{*payload})
	require.NoError(t, err)
	expectedRootHashHex := "d2536303495a9325037d247cbb2b9be4d6cb3465986ea2c4481d8770ff16b6b0"
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
	expectedRootHashHex := "58042aca145b316263581d1789a1fc50ac2844f1df08cb006d0849e788f6b754"
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
		paths = append(paths, utils.PathByUint16(uint16(i)))
		temp := rng.next()
		payload := utils.LightPayload(temp, temp)
		payloads = append(payloads, *payload)
	}
	updatedTrie, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths, payloads)
	require.NoError(t, err)
	expectedRootHashHex := "99e12f9f9406ddd7b9b98bb15c6d643be3ae49e78098713fa00409fee634c065"
	require.Equal(t, expectedRootHashHex, hex.EncodeToString(updatedTrie.RootHash()))
}

// TestUpdateTrie tests whether iteratively updating a Trie matches the formal specification.
// The expected root hashes are coming from a reference implementation in python and is hard-coded here.
func Test_UpdateTrie(t *testing.T) {
	expectedRootHashes := []string{
		"d163b7f3de8ac52f94821f9ed0d90586beccfdd43a354ee09c877b1c2d9e2426",
		"1b20ecb5ee4d86e6160778e7e589978612ca41f3dd4be4c0a62f78411b420988",
		"5dfe9d5c1d6b5bb2dd637ce29169fb2be2c923ef7faf93931c53364e060af16e",
		"2b0fcefa295024f20ef057bb8148288e6b75c07abc4ca3ec7c5b63dedd11fcc7",
		"f3ae74b49f7172ef5b7c2013751a38c3d2aa13ea9524f86efbb66b8a28cabfea",
		"0299b3b008429b6748f0ce2c0969da9cecceea27d4a4dd322e09330ce1a4e124",
		"ca584ffa4f9a12cfeb80db84035e10927aaa77ef3b9e045070fd2aee7f00b80b",
		"30ae55faccf75a406d946b5ea0ea3f3f0d15b88d916b5d3c50f368aeef454282",
		"f1b2bc0d5cbab3c9683c1c4660145e33e38120cc96d5a34104b08513bbdf9d9b",
		"86631d17047349631b744dbbf652db5f0d4aec4889e36af9dfdfd51b02df019a",
		"0b592f1c0f0b06f169b378cecede79ec80769fb943220700771d30e71d30d679",
		"ac46074bf22e83f62b048f6ac65c98db85ee6a441e14394bdf87e7e2714fd6d3",
		"8fadb2a3a4013a912078b90995f0ad9d4c8aabc318d9ee74bd6a6cdf6e923fe4",
		"ded726b94a953cfa2e5bae36bc8bcb805f34854007c58f0674487137cad43cba",
		"9b1f623553614dc91aa61f782d46a8cd48f4a59a078f21daa41b215e07f2679d",
		"58042aca145b316263581d1789a1fc50ac2844f1df08cb006d0849e788f6b754",
		"4dbff09d0523fe95987052f93d233471f8e67292f5b9cdffcbe0f0cc301b570d",
		"c9163bcc4adf6ca2e4f902489aae08d68e82421d1aaadf2f3cdca7e6f29d3a80",
		"81ae9abd57c623cf801eb3b63837274b254506c4d9e2e8e32128f6229cb38095",
		"9ab50f4c7cb985ef435e5d8ed2205654d125a477062c86d15e96187f8326e5a6",
	}

	// Make new Trie (independently of MForest):
	emptyTrie, err := trie.NewEmptyMTrie(ReferenceImplPathByteSize)
	require.NoError(t, err)

	// allocate single random register
	rng := &LinearCongruentialGenerator{seed: 0}
	path := utils.PathByUint16(rng.next())
	temp := rng.next()
	payload := utils.LightPayload(temp, temp)
	updatedTrie, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, []ledger.Path{path}, []ledger.Payload{*payload})
	require.NoError(t, err)
	expectedRootHashHex := "d163b7f3de8ac52f94821f9ed0d90586beccfdd43a354ee09c877b1c2d9e2426"
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
	expectedRootHashHex := "caebf1bec988450027a9a0155be7bc68f493c5037f8087537b17f9d9bae6e81d"
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
		path := utils.PathByUint16(rng.next())
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
