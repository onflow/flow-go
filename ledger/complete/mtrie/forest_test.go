package mtrie

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common"
	"github.com/onflow/flow-go/ledger/common/encoding"
	"github.com/onflow/flow-go/ledger/common/utils"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/ledger/partial/ptrie"
	"github.com/onflow/flow-go/module/metrics"
)

// TestTrieOperations tests adding removing and retrieving Trie from Forest
func TestTrieOperations(t *testing.T) {
	pathByteSize := 2 // path size of 16 bits

	dir, err := ioutil.TempDir("", "test-mtrie-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	forest, err := NewForest(pathByteSize, dir, 5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	// Make new Trie (independently of MForest):
	nt, err := trie.NewEmptyMTrie(pathByteSize)
	require.NoError(t, err)
	p1 := pathByUint8s([]uint8{uint8(53), uint8(74)}, pathByteSize)
	v1 := payloadBySlices([]byte{'A'}, []byte{'A'})

	updatedTrie, err := trie.NewTrieWithUpdatedRegisters(nt, []ledger.Path{p1}, []ledger.Payload{*v1})
	require.NoError(t, err)

	// Add trie
	err = forest.AddTrie(updatedTrie)
	require.NoError(t, err)

	// Get trie
	retnt, err := forest.GetTrie(updatedTrie.RootHash())
	require.NoError(t, err)
	require.True(t, bytes.Equal(retnt.RootHash(), updatedTrie.RootHash()))
	require.Equal(t, forest.Size(), 2)

	// Remove trie
	forest.RemoveTrie(updatedTrie.RootHash())
	require.Equal(t, forest.Size(), 1)
}

// TestTrieUpdate updates the empty trie with some values and verifies that the
// written values can be retrieved from the updated trie.
func TestTrieUpdate(t *testing.T) {
	pathByteSize := 2 // path size of 16 bits

	dir, err := ioutil.TempDir("", "test-mtrie-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	metricsCollector := &metrics.NoopCollector{}
	forest, err := NewForest(pathByteSize, dir, 5, metricsCollector, nil)
	require.NoError(t, err)
	rootHash := forest.GetEmptyRootHash()

	p1 := pathByUint8s([]uint8{uint8(53), uint8(74)}, pathByteSize)
	v1 := payloadBySlices([]byte{'A'}, []byte{'A'})

	paths := []ledger.Path{p1}
	payloads := []*ledger.Payload{v1}
	update := &ledger.TrieUpdate{RootHash: rootHash, Paths: paths, Payloads: payloads}
	updatedRoot, err := forest.Update(update)
	require.NoError(t, err)

	read := &ledger.TrieRead{RootHash: updatedRoot, Paths: paths}
	retPayloads, err := forest.Read(read)
	require.NoError(t, err)
	require.True(t, bytes.Equal(encoding.EncodePayload(retPayloads[0]), encoding.EncodePayload(payloads[0])))
}

// TestLeftEmptyInsert tests inserting a new value into an empty sub-trie:
//   1. we first construct a baseTrie holding a couple of values on the right branch [~]
//   2. we update a previously non-existent register on the left branch (X)
// We verify that values for _all_ paths in the updated Trie have correct payloads
func TestLeftEmptyInsert(t *testing.T) {
	//////////////////////
	//     insert X     //
	//       ()         //
	//      /  \        //
	//    (X)  [~]      //
	//////////////////////
	pathByteSize := 2 // path size of 16 bits
	dir, err := ioutil.TempDir("", "test-mtrie-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	forest, err := NewForest(pathByteSize, dir, 5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	// path: 1000...
	p1 := pathByUint8s([]uint8{uint8(129), uint8(1)}, pathByteSize)
	v1 := payloadBySlices([]byte{'A'}, []byte{'A'})

	// path: 1100...
	p2 := pathByUint8s([]uint8{uint8(193), uint8(1)}, pathByteSize)
	v2 := payloadBySlices([]byte{'B'}, []byte{'B'})

	paths := []ledger.Path{p1, p2}
	payloads := []*ledger.Payload{v1, v2}
	update := &ledger.TrieUpdate{RootHash: forest.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
	baseRoot, err := forest.Update(update)
	require.NoError(t, err)

	baseTrie, err := forest.GetTrie(baseRoot)
	require.NoError(t, err)
	require.Equal(t, baseTrie.MaxDepth(), uint16(2))
	require.Equal(t, baseTrie.AllocatedRegCount(), uint64(2))
	// resulting base trie:
	// 16: (path:, hash:b07...4bd)[]
	// 		15: (path:, hash:961...af6)[1]
	// 			14: (path:1000000100000001, hash:7b6...095)[10]
	// 			14: (path:1100000100000001, hash:a24...f52)[11]
	fmt.Println("BASE TRIE:")
	fmt.Println(baseTrie.String())

	p3 := pathByUint8s([]uint8{uint8(1), uint8(1)}, pathByteSize)
	v3 := payloadBySlices([]byte{'C'}, []byte{'C'})

	paths = []ledger.Path{p3}
	payloads = []*ledger.Payload{v3}
	update = &ledger.TrieUpdate{RootHash: baseTrie.RootHash(), Paths: paths, Payloads: payloads}
	updatedRoot, err := forest.Update(update)
	require.NoError(t, err)

	updatedTrie, err := forest.GetTrie(updatedRoot)
	require.NoError(t, err)
	require.Equal(t, updatedTrie.MaxDepth(), uint16(2))
	require.Equal(t, updatedTrie.AllocatedRegCount(), uint64(3))
	// expected updated Trie:
	// 16: (path:, hash:ae6...645)[]
	// 		15: (path:0000000100000001, hash:0ae...3ee)[0]
	// 		15: (path:, hash:961...af6)[1]
	// 			14: (path:1000000100000001, hash:7b6...095)[10]
	// 			14: (path:1100000100000001, hash:a24...f52)[11]
	fmt.Println("UPDATED TRIE:")
	fmt.Println(updatedTrie.String())
	paths = []ledger.Path{p1, p2, p3}
	payloads = []*ledger.Payload{v1, v2, v3}
	read := &ledger.TrieRead{RootHash: updatedRoot, Paths: paths}
	retPayloads, err := forest.Read(read)
	require.NoError(t, err)
	for i := range paths {
		require.True(t, bytes.Equal(encoding.EncodePayload(retPayloads[i]), encoding.EncodePayload(payloads[i])))
	}
}

// TestLeftEmptyInsert tests inserting a new value into an empty sub-trie:
//   1. we first construct a baseTrie holding a couple of values on the left branch [~]
//   2. we update a previously non-existent register on the right branch (X)
// We verify that values for _all_ paths in the updated Trie have correct payloads
func TestRightEmptyInsert(t *testing.T) {
	///////////////////////
	//     insert X      //
	//       ()          //
	//      /  \         //
	//    [~]  (X)       //
	///////////////////////
	pathByteSize := 2 // path size of 16 bits
	dir, err := ioutil.TempDir("", "test-mtrie-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	forest, err := NewForest(pathByteSize, dir, 5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	// path: 0000...
	p1 := pathByUint8s([]uint8{uint8(1), uint8(1)}, pathByteSize)
	v1 := payloadBySlices([]byte{'A'}, []byte{'A'})

	// path: 0100...
	p2 := pathByUint8s([]uint8{uint8(64), uint8(1)}, pathByteSize)
	v2 := payloadBySlices([]byte{'B'}, []byte{'B'})

	paths := []ledger.Path{p1, p2}
	payloads := []*ledger.Payload{v1, v2}
	update := &ledger.TrieUpdate{RootHash: forest.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
	baseRoot, err := forest.Update(update)
	require.NoError(t, err)

	baseTrie, err := forest.GetTrie(baseRoot)
	require.NoError(t, err)
	require.Equal(t, baseTrie.MaxDepth(), uint16(2))
	require.Equal(t, baseTrie.AllocatedRegCount(), uint64(2))
	// resulting base trie:
	// 16: (path:, hash:c6c...e2e)[]
	// 		15: (path:, hash:abc...895)[0]
	// 			14: (path:0000000100000001, hash:2d9...1c8)[00]
	// 			14: (path:0100000000000001, hash:61e...d72)[01]
	fmt.Println("BASE TRIE:")
	fmt.Println(baseTrie.String())

	// path: 1000...
	p3 := pathByUint8s([]uint8{uint8(129), uint8(1)}, pathByteSize)
	v3 := payloadBySlices([]byte{'C'}, []byte{'C'})

	paths = []ledger.Path{p3}
	payloads = []*ledger.Payload{v3}
	update = &ledger.TrieUpdate{RootHash: baseTrie.RootHash(), Paths: paths, Payloads: payloads}
	updatedRoot, err := forest.Update(update)
	require.NoError(t, err)

	updatedTrie, err := forest.GetTrie(updatedRoot)
	require.NoError(t, err)
	require.Equal(t, updatedTrie.MaxDepth(), uint16(2))
	require.Equal(t, updatedTrie.AllocatedRegCount(), uint64(3))
	// expected updated Trie:
	// 16: (path:, hash:e21...6cc)[]
	// 		15: (path:, hash:abc...895)[0]
	// 			14: (path:0000000100000001, hash:2d9...1c8)[00]
	// 			14: (path:0100000000000001, hash:61e...d72)[01]
	// 		15: (path:1000000100000001, hash:c9a...33e)[1]
	fmt.Println("UPDATED TRIE:")
	fmt.Println(updatedTrie.String())

	paths = []ledger.Path{p1, p2, p3}
	payloads = []*ledger.Payload{v1, v2, v3}
	read := &ledger.TrieRead{RootHash: updatedRoot, Paths: paths}
	retPayloads, err := forest.Read(read)
	require.NoError(t, err)
	for i := range paths {
		require.True(t, bytes.Equal(encoding.EncodePayload(retPayloads[i]), encoding.EncodePayload(payloads[i])))
	}
}

// TestExpansionInsert tests inserting a new value into a populated sub-trie, where a
// leaf (holding a single value) would be replaced by an expanded sub-trie holding multiple value
//   1. we first construct a baseTrie holding a couple of values on the right branch [~]
//   2. we update a previously non-existent register on the right branch turning [~] to [~']
// We verify that values for _all_ paths in the updated Trie have correct payloads
func TestExpansionInsert(t *testing.T) {
	////////////////////////
	// modify [~] -> [~'] //
	//       ()           //
	//      /  \          //
	//         [~]        //
	////////////////////////

	pathByteSize := 2 // path size of 16 bits
	dir, err := ioutil.TempDir("", "test-mtrie-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	forest, err := NewForest(pathByteSize, dir, 5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	// path: 100000...
	p1 := pathByUint8s([]uint8{uint8(129), uint8(1)}, pathByteSize)
	v1 := payloadBySlices([]byte{'A'}, []byte{'A'})

	paths := []ledger.Path{p1}
	payloads := []*ledger.Payload{v1}
	update := &ledger.TrieUpdate{RootHash: forest.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
	baseRoot, err := forest.Update(update)
	require.NoError(t, err)

	baseTrie, err := forest.GetTrie(baseRoot)
	require.NoError(t, err)
	require.Equal(t, baseTrie.MaxDepth(), uint16(1))
	require.Equal(t, baseTrie.AllocatedRegCount(), uint64(1))
	// resulting base trie:
	// 16: (path:, hash:737...2a0)[]
	// 		15: (path:1000000100000001, hash:c0e...0ca)[1]
	fmt.Println("BASE TRIE:")
	fmt.Println(baseTrie.String())

	// path: 1000001...
	p2 := pathByUint8s([]uint8{uint8(130), uint8(1)}, pathByteSize)
	v2 := payloadBySlices([]byte{'B'}, []byte{'B'})

	paths = []ledger.Path{p2}
	payloads = []*ledger.Payload{v2}
	update = &ledger.TrieUpdate{RootHash: baseTrie.RootHash(), Paths: paths, Payloads: payloads}
	updatedRoot, err := forest.Update(update)
	require.NoError(t, err)

	updatedTrie, err := forest.GetTrie(updatedRoot)
	require.NoError(t, err)
	require.Equal(t, updatedTrie.MaxDepth(), uint16(7))
	require.Equal(t, updatedTrie.AllocatedRegCount(), uint64(2))
	// expected updated Trie:
	// 16: (path:, hash:1a6...5c3)[]
	// 		15: (path:, hash:810...713)[1]
	// 			14: (path:, hash:1ad...0a8)[10]
	// 				13: (path:, hash:b61...3d2)[100]
	// 					12: (path:, hash:966...115)[1000]
	// 						11: (path:, hash:0e2...f3f)[10000]
	// 							10: (path:, hash:10b...e9a)[100000]
	// 								9: (path:1000000100000001, hash:973...101)[1000000]
	// 								9: (path:1000001000000001, hash:839...e32)[1000001]
	fmt.Println("UPDATED TRIE:")
	fmt.Println(updatedTrie.String())

	paths = []ledger.Path{p1, p2}
	payloads = []*ledger.Payload{v1, v2}
	read := &ledger.TrieRead{RootHash: updatedRoot, Paths: paths}
	retPayloads, err := forest.Read(read)
	require.NoError(t, err)
	for i := range paths {
		require.True(t, bytes.Equal(encoding.EncodePayload(retPayloads[i]), encoding.EncodePayload(payloads[i])))
	}
}

// TestFullHouseInsert tests inserting a new value into a populated sub-trie, where a
// leaf's value is overridden _and_ further values are added which all fall into a subtree that
// replaces the leaf:
//   1. we first construct a baseTrie holding a couple of values on the right branch [~]
//   2. we update a previously non-existent register on the right branch turning [~] to [~']
// We verify that values for _all_ paths in the updated Trie have correct payloads
func TestFullHouseInsert(t *testing.T) {
	///////////////////////
	//   insert ~1<X<~2  //
	//       ()          //
	//      /  \         //
	//    [~1]  [~2]     //
	///////////////////////

	pathByteSize := 2 // path size of 16 bits
	dir, err := ioutil.TempDir("", "test-mtrie-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	forest, err := NewForest(pathByteSize, dir, 5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	// paths p0 forms [~1]; p1 and p2 form [~2]
	// path: 0100...
	p0 := pathByUint8s([]uint8{uint8(64), uint8(1)}, pathByteSize)
	v0 := payloadBySlices([]byte{'0'}, []byte{'0'})
	// path: 1000...
	p1 := pathByUint8s([]uint8{uint8(129), uint8(1)}, pathByteSize)
	v1 := payloadBySlices([]byte{'A'}, []byte{'A'})

	// path: 1100...
	p2 := pathByUint8s([]uint8{uint8(193), uint8(1)}, pathByteSize)
	v2 := payloadBySlices([]byte{'B'}, []byte{'B'})

	paths := []ledger.Path{p0, p1, p2}
	payloads := []*ledger.Payload{v0, v1, v2}
	update := &ledger.TrieUpdate{RootHash: forest.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
	baseRoot, err := forest.Update(update)
	require.NoError(t, err)

	baseTrie, err := forest.GetTrie(baseRoot)
	require.NoError(t, err)
	require.Equal(t, baseTrie.MaxDepth(), uint16(2))
	require.Equal(t, baseTrie.AllocatedRegCount(), uint64(3))
	// expected trie:
	// 16: (path:, hash:9f5...00e)[]
	// 		15: (path:0100000000000001, hash:590...108)[0]
	// 		15: (path:, hash:961...af6)[1]
	// 			14: (path:1000000100000001, hash:7b6...095)[10]
	// 			14: (path:1100000100000001, hash:a24...f52)[11]
	fmt.Println("BASE TRIE:")
	fmt.Println(baseTrie.String())

	// we update value for path p1 and in addition add p3 that has the same prefix `10` as p0
	v1 = payloadBySlices([]byte{'X'}, []byte{'X'})

	// path: 1010...
	p3 := pathByUint8s([]uint8{uint8(160), uint8(1)}, pathByteSize)
	v3 := payloadBySlices([]byte{'C'}, []byte{'C'})

	paths = []ledger.Path{p1, p3}
	payloads = []*ledger.Payload{v1, v3}
	update = &ledger.TrieUpdate{RootHash: baseTrie.RootHash(), Paths: paths, Payloads: payloads}
	updatedRoot, err := forest.Update(update)
	require.NoError(t, err)

	updatedTrie, err := forest.GetTrie(updatedRoot)
	require.NoError(t, err)
	require.Equal(t, updatedTrie.MaxDepth(), uint16(3))
	require.Equal(t, updatedTrie.AllocatedRegCount(), uint64(4))
	// expected trie:
	// 16: (path:, hash:e2b...13e)[]
	// 		15: (path:0100000000000001, hash:590...108)[0]
	// 		15: (path:, hash:e5d...1b7)[1]
	// 			14: (path:, hash:e71...3c1)[10]
	// 				13: (path:1000000100000001, hash:bb7...85f)[100]
	// 				13: (path:1010000000000001, hash:8bd...428)[101]
	// 			14: (path:1100000100000001, hash:a24...f52)[11]
	fmt.Println("UPDATED TRIE:")
	fmt.Println(updatedTrie.String())

	paths = []ledger.Path{p1, p2, p3}
	payloads = []*ledger.Payload{v1, v2, v3}
	read := &ledger.TrieRead{RootHash: updatedRoot, Paths: paths}
	retPayloads, err := forest.Read(read)
	require.NoError(t, err)
	for i := range paths {
		require.True(t, bytes.Equal(encoding.EncodePayload(retPayloads[i]), encoding.EncodePayload(payloads[i])))
	}
}

// TestLeafInsert inserts two keys, which only differ in their last bit.
// I.e. the trie needs to be expanded to its hull depth
// We verify that values for _all_ paths in the updated Trie have correct payloads
func TestLeafInsert(t *testing.T) {
	///////////////////////
	//   insert 1, 2     //
	//       ()          //
	//      /  \         //
	//     ()   ...      //
	//          /  \     //
	//         ()  ()    //
	///////////////////////
	pathByteSize := 2 // path size of 16 bits
	dir, err := ioutil.TempDir("", "test-mtrie-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	forest, err := NewForest(pathByteSize, dir, 5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	// path: 0000000100000000
	p1 := pathByUint8s([]uint8{uint8(1), uint8(0)}, pathByteSize)
	v1 := payloadBySlices([]byte{'A'}, []byte{'A'})

	// path: 0000000100000001
	p2 := pathByUint8s([]uint8{uint8(1), uint8(1)}, pathByteSize)
	v2 := payloadBySlices([]byte{'B'}, []byte{'B'})

	paths := []ledger.Path{p1, p2}
	payloads := []*ledger.Payload{v1, v2}
	update := &ledger.TrieUpdate{RootHash: forest.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
	updatedRoot, err := forest.Update(update)
	require.NoError(t, err)

	updatedTrie, err := forest.GetTrie(updatedRoot)
	require.NoError(t, err)
	require.Equal(t, updatedTrie.MaxDepth(), uint16(16))
	require.Equal(t, updatedTrie.AllocatedRegCount(), uint64(2))
	// expected trie:
	// 16: (path:, hash:63c...d7f)[]
	// 		15: (path:, hash:67f...6c7)[0]
	//  		14: (path:, hash:4dd...cb4)[00]
	//					...
	//						0: (path:0000000100000000, hash:fa8...263)[0000000100000000]
	//						0: (path:0000000100000001, hash:605...57d)[0000000100000001]
	fmt.Println("TRIE:")
	fmt.Println(updatedTrie.String())

	read := &ledger.TrieRead{RootHash: updatedRoot, Paths: paths}
	retPayloads, err := forest.Read(read)
	require.NoError(t, err)
	for i := range paths {
		require.True(t, bytes.Equal(encoding.EncodePayload(retPayloads[i]), encoding.EncodePayload(payloads[i])))
	}
}

// TestOverrideValue overrides an existing value in the trie (without any expansion)
// We verify that values for _all_ paths in the updated Trie have correct payloads
func TestOverrideValue(t *testing.T) {
	pathByteSize := 2 // path size of 16 bits
	dir, err := ioutil.TempDir("", "test-mtrie-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	forest, err := NewForest(pathByteSize, dir, 5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	// path: 1000...
	p1 := pathByUint8s([]uint8{uint8(53), uint8(74)}, pathByteSize)
	v1 := payloadBySlices([]byte{'A'}, []byte{'A'})

	// path: 0111...
	p2 := pathByUint8s([]uint8{uint8(116), uint8(129)}, pathByteSize)
	v2 := payloadBySlices([]byte{'B'}, []byte{'B'})

	paths := []ledger.Path{p1, p2}
	payloads := []*ledger.Payload{v1, v2}
	update := &ledger.TrieUpdate{RootHash: forest.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
	baseRoot, err := forest.Update(update)
	require.NoError(t, err)

	// path: 1000...
	p3 := pathByUint8s([]uint8{uint8(53), uint8(74)}, pathByteSize)
	v3 := payloadBySlices([]byte{'C'}, []byte{'C'})

	paths = []ledger.Path{p3}
	payloads = []*ledger.Payload{v3}
	update = &ledger.TrieUpdate{RootHash: baseRoot, Paths: paths, Payloads: payloads}
	updatedRoot, err := forest.Update(update)
	require.NoError(t, err)

	read := &ledger.TrieRead{RootHash: updatedRoot, Paths: paths}
	retPayloads, err := forest.Read(read)
	require.NoError(t, err)
	require.True(t, bytes.Equal(encoding.EncodePayload(retPayloads[0]), encoding.EncodePayload(payloads[0])))

}

// TestDuplicateOverride tests behaviour when the updates contain two different payloads for the
// same path. I.e. we update with (p0, v0) and (p0, v1)
// We expect that the _last_ written value is persisted in the Trie
func TestDuplicateOverride(t *testing.T) {
	pathByteSize := 2 // path size of 16 bits
	dir, err := ioutil.TempDir("", "test-mtrie-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	forest, err := NewForest(pathByteSize, dir, 5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	// path: 1000...
	p0 := pathByUint8s([]uint8{uint8(53), uint8(74)}, pathByteSize)
	v0 := payloadBySlices([]byte{'A'}, []byte{'A'})
	paths := []ledger.Path{p0}
	payloads := []*ledger.Payload{v0}
	update := &ledger.TrieUpdate{RootHash: forest.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
	baseRoot, err := forest.Update(update)
	require.NoError(t, err)

	v1 := payloadBySlices([]byte{'B'}, []byte{'B'})
	v2 := payloadBySlices([]byte{'C'}, []byte{'C'})
	paths = []ledger.Path{p0, p0}
	payloads = []*ledger.Payload{v1, v2}
	update = &ledger.TrieUpdate{RootHash: baseRoot, Paths: paths, Payloads: payloads}
	updatedRoot, err := forest.Update(update)
	require.NoError(t, err)

	paths = []ledger.Path{p0}
	read := &ledger.TrieRead{RootHash: updatedRoot, Paths: paths}
	retPayloads, err := forest.Read(read)
	require.NoError(t, err)
	require.True(t, bytes.Equal(encoding.EncodePayload(retPayloads[0]), encoding.EncodePayload(v2)))

}

// TestReadSafety check if payload returned from a forest are safe against modification - ie. copy of the data
// is returned, instead of a slice
func TestReadSafety(t *testing.T) {
	pathByteSize := 2 // path size of 16 bits
	dir, err := ioutil.TempDir("", "test-mtrie-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	forest, err := NewForest(pathByteSize, dir, 5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	// path: 1000...
	p0 := pathByUint8s([]uint8{uint8(53), uint8(74)}, pathByteSize)
	v0 := payloadBySlices([]byte{'A'}, []byte{'A'})
	paths := []ledger.Path{p0}
	payloads := []*ledger.Payload{v0}
	update := &ledger.TrieUpdate{RootHash: forest.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
	baseRoot, err := forest.Update(update)
	require.NoError(t, err)

	read := &ledger.TrieRead{RootHash: baseRoot, Paths: paths}
	data, err := forest.Read(read)
	require.NoError(t, err)

	require.Len(t, data, 1)
	require.Equal(t, v0, data[0])

	// modify returned slice
	data[0].Value = []byte("new value")

	// read again
	data2, err := forest.Read(read)
	require.NoError(t, err)
	require.Len(t, data2, 1)
	require.Equal(t, v0, data2[0])
}

// TestUpdateWithWrongPathSize verifies that attempting to update a trie with a wrong path size
func TestUpdateWithWrongPathSize(t *testing.T) {
	pathByteSize := 2 // path size of 16 bits
	dir, err := ioutil.TempDir("", "test-mtrie-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	forest, err := NewForest(pathByteSize, dir, 5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	// short key
	p1 := pathByUint8s([]uint8{uint8(1)}, 1)
	v1 := payloadBySlices([]byte{'A'}, []byte{'A'})
	paths := []ledger.Path{p1}
	payloads := []*ledger.Payload{v1}

	update := &ledger.TrieUpdate{RootHash: forest.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
	_, err = forest.Update(update)
	require.Error(t, err)

	// long key
	p2 := pathByUint8s([]uint8{uint8(1)}, 33)
	v2 := payloadBySlices([]byte{'A'}, []byte{'A'})
	paths = []ledger.Path{p2}
	payloads = []*ledger.Payload{v2}

	update = &ledger.TrieUpdate{RootHash: forest.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
	_, err = forest.Update(update)
	require.Error(t, err)
}

// TestReadOrder tests that payloads from reading a trie are delivered in the order as specified by the paths
func TestReadOrder(t *testing.T) {
	pathByteSize := 2 // path size of 16 bits
	dir, err := ioutil.TempDir("", "test-mtrie-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	forest, err := NewForest(pathByteSize, dir, 5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	p1 := pathByUint8s([]uint8{uint8(116), uint8(74)}, pathByteSize)
	v1 := payloadBySlices([]byte{'A'}, []byte{'A'})

	p2 := pathByUint8s([]uint8{uint8(53), uint8(129)}, pathByteSize)
	v2 := payloadBySlices([]byte{'B'}, []byte{'B'})

	paths := []ledger.Path{p1, p2}
	payloads := []*ledger.Payload{v1, v2}
	update := &ledger.TrieUpdate{RootHash: forest.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
	testRoot, err := forest.Update(update)
	require.NoError(t, err)

	read := &ledger.TrieRead{RootHash: testRoot, Paths: []ledger.Path{p1, p2}}
	retPayloads, err := forest.Read(read)
	require.NoError(t, err)
	require.True(t, bytes.Equal(encoding.EncodePayload(retPayloads[0]), encoding.EncodePayload(payloads[0])))
	require.True(t, bytes.Equal(encoding.EncodePayload(retPayloads[1]), encoding.EncodePayload(payloads[1])))

	read = &ledger.TrieRead{RootHash: testRoot, Paths: []ledger.Path{p2, p1}}
	retPayloads, err = forest.Read(read)
	require.NoError(t, err)
	require.True(t, bytes.Equal(encoding.EncodePayload(retPayloads[1]), encoding.EncodePayload(payloads[0])))
	require.True(t, bytes.Equal(encoding.EncodePayload(retPayloads[0]), encoding.EncodePayload(payloads[1])))
}

// TestMixRead tests reading a mixture of set and unset registers.
// We expect the default payload (nil) to be returned for unset registers.
func TestMixRead(t *testing.T) {
	pathByteSize := 2 // path size of 16 bits
	dir, err := ioutil.TempDir("", "test-mtrie-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	forest, err := NewForest(pathByteSize, dir, 5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	// path: 01111101...
	p1 := pathByUint8s([]uint8{uint8(125), uint8(23)}, pathByteSize)
	v1 := payloadBySlices([]byte{'A'}, []byte{'A'})

	// path: 10110010...
	p2 := pathByUint8s([]uint8{uint8(178), uint8(152)}, pathByteSize)
	v2 := payloadBySlices([]byte{'B'}, []byte{'B'})

	paths := []ledger.Path{p1, p2}
	payloads := []*ledger.Payload{v1, v2}

	update := &ledger.TrieUpdate{RootHash: forest.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
	baseRoot, err := forest.Update(update)
	require.NoError(t, err)

	// path: 01101110...
	p3 := pathByUint8s([]uint8{uint8(110), uint8(48)}, pathByteSize)
	v3 := ledger.EmptyPayload()

	// path: 00010111...
	p4 := pathByUint8s([]uint8{uint8(23), uint8(82)}, pathByteSize)
	v4 := ledger.EmptyPayload()

	readPaths := []ledger.Path{p1, p2, p3, p4}
	expectedPayloads := []*ledger.Payload{v1, v2, v3, v4}

	read := &ledger.TrieRead{RootHash: baseRoot, Paths: readPaths}
	retPayloads, err := forest.Read(read)
	require.NoError(t, err)
	for i := range paths {
		require.True(t, bytes.Equal(encoding.EncodePayload(retPayloads[i]), encoding.EncodePayload(expectedPayloads[i])))
	}
}

// TestReadWithDuplicatedKeys reads a the values for two keys, where both keys have the same value.
// We expect that we receive the respective value twice in the return.
func TestReadWithDuplicatedKeys(t *testing.T) {
	pathByteSize := 2 // path size of 16 bits
	dir, err := ioutil.TempDir("", "test-mtrie-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	forest, err := NewForest(pathByteSize, dir, 5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	p1 := pathByUint8s([]uint8{uint8(53), uint8(74)}, pathByteSize)
	v1 := payloadBySlices([]byte{'A'}, []byte{'A'})
	p2 := pathByUint8s([]uint8{uint8(116), uint8(129)}, pathByteSize)
	v2 := payloadBySlices([]byte{'B'}, []byte{'B'})
	p3 := pathByUint8s([]uint8{uint8(53), uint8(74)}, pathByteSize)

	paths := []ledger.Path{p1, p2}
	payloads := []*ledger.Payload{v1, v2}
	update := &ledger.TrieUpdate{RootHash: forest.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
	updatedRoot, err := forest.Update(update)
	require.NoError(t, err)

	paths = []ledger.Path{p1, p2, p3}
	expectedPayloads := []*ledger.Payload{v1, v2, v1}
	read := &ledger.TrieRead{RootHash: updatedRoot, Paths: paths}
	retPayloads, err := forest.Read(read)
	require.NoError(t, err)
	for i := range paths {
		require.True(t, bytes.Equal(encoding.EncodePayload(retPayloads[i]), encoding.EncodePayload(expectedPayloads[i])))
	}
}

// TestReadNonExistingPath tests reading an unset path.
func TestReadNonExistingPath(t *testing.T) {
	pathByteSize := 2 // path size of 16 bits
	dir, err := ioutil.TempDir("", "test-mtrie-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	forest, err := NewForest(pathByteSize, dir, 5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	p1 := pathByUint8s([]uint8{uint8(53), uint8(74)}, pathByteSize)
	v1 := payloadBySlices([]byte{'A'}, []byte{'A'})
	paths := []ledger.Path{p1}
	payloads := []*ledger.Payload{v1}

	update := &ledger.TrieUpdate{RootHash: forest.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
	updatedRoot, err := forest.Update(update)
	require.NoError(t, err)

	p2 := pathByUint8s([]uint8{uint8(116), uint8(129)}, pathByteSize)
	read := &ledger.TrieRead{RootHash: updatedRoot, Paths: []ledger.Path{p2}}
	retPayloads, err := forest.Read(read)
	require.NoError(t, err)
	require.True(t, retPayloads[0].IsEmpty())
}

// TestReadWithWrongPathSize verifies that attempting to read a trie with wrong path size
func TestReadWithWrongPathSize(t *testing.T) {
	pathByteSize := 2 // path size of 16 bits
	dir, err := ioutil.TempDir("", "test-mtrie-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	forest, err := NewForest(pathByteSize, dir, 5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	// setup
	p1 := pathByUint8s([]uint8{uint8(1)}, pathByteSize)
	v1 := payloadBySlices([]byte{'A'}, []byte{'A'})
	paths := []ledger.Path{p1}
	payloads := []*ledger.Payload{v1}
	update := &ledger.TrieUpdate{RootHash: forest.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
	updatedRoot, err := forest.Update(update)
	require.NoError(t, err)

	// key too short
	p2 := pathByUint8s([]uint8{uint8(1)}, 1)
	read := &ledger.TrieRead{RootHash: updatedRoot, Paths: []ledger.Path{p2}}
	_, err = forest.Read(read)
	require.Error(t, err)

	// key too long
	p3 := pathByUint8s([]uint8{uint8(1)}, 33)
	read = &ledger.TrieRead{RootHash: updatedRoot, Paths: []ledger.Path{p3}}
	_, err = forest.Read(read)
	require.Error(t, err)
}

// // TODO test read (multiple non exist in a branch)
// // [AlexH] doesn't TestMixRead do this test?

// TestForkingUpdates updates a base trie in two different ways. We expect
// that for each update, a new trie is added to the forest preserving the
// updated values independently of the other update.
func TestForkingUpdates(t *testing.T) {
	pathByteSize := 2 // path size of 16 bits
	dir, err := ioutil.TempDir("", "test-mtrie-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	forest, err := NewForest(pathByteSize, dir, 5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	p1 := pathByUint8s([]uint8{uint8(53), uint8(74)}, pathByteSize)
	v1 := payloadBySlices([]byte{'A'}, []byte{'A'})
	p2 := pathByUint8s([]uint8{uint8(116), uint8(129)}, pathByteSize)
	v2 := payloadBySlices([]byte{'B'}, []byte{'B'})
	paths := []ledger.Path{p1, p2}
	payloads := []*ledger.Payload{v1, v2}
	update := &ledger.TrieUpdate{RootHash: forest.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
	baseRoot, err := forest.Update(update)
	require.NoError(t, err)

	// update baseTrie -> updatedTrieA
	v1a := payloadBySlices([]byte{'C'}, []byte{'C'})
	p3a := pathByUint8s([]uint8{uint8(116), uint8(22)}, pathByteSize)
	v3a := payloadBySlices([]byte{'D'}, []byte{'D'})
	pathsA := []ledger.Path{p1, p3a}
	payloadsA := []*ledger.Payload{v1a, v3a}
	updateA := &ledger.TrieUpdate{RootHash: baseRoot, Paths: pathsA, Payloads: payloadsA}
	updatedRootA, err := forest.Update(updateA)
	require.NoError(t, err)

	// update baseTrie -> updatedTrieB
	v1b := payloadBySlices([]byte{'E'}, []byte{'E'})
	p3b := pathByUint8s([]uint8{uint8(116), uint8(22)}, pathByteSize)
	v3b := payloadBySlices([]byte{'F'}, []byte{'F'})
	pathsB := []ledger.Path{p1, p3b}
	payloadsB := []*ledger.Payload{v1b, v3b}
	updateB := &ledger.TrieUpdate{RootHash: baseRoot, Paths: pathsB, Payloads: payloadsB}
	updatedRootB, err := forest.Update(updateB)
	require.NoError(t, err)

	// Verify payloads are preserved
	read := &ledger.TrieRead{RootHash: baseRoot, Paths: paths}
	retPayloads, err := forest.Read(read) // reading from original Trie
	require.NoError(t, err)
	for i := range paths {
		require.True(t, bytes.Equal(encoding.EncodePayload(retPayloads[i]), encoding.EncodePayload(payloads[i])))
	}

	readA := &ledger.TrieRead{RootHash: updatedRootA, Paths: pathsA}
	retPayloads, err = forest.Read(readA) // reading from updatedTrieA
	require.NoError(t, err)
	for i := range paths {
		require.True(t, bytes.Equal(encoding.EncodePayload(retPayloads[i]), encoding.EncodePayload(payloadsA[i])))
	}

	readB := &ledger.TrieRead{RootHash: updatedRootB, Paths: pathsB}
	retPayloads, err = forest.Read(readB) // reading from updatedTrieB
	require.NoError(t, err)
	for i := range paths {
		require.True(t, bytes.Equal(encoding.EncodePayload(retPayloads[i]), encoding.EncodePayload(payloadsB[i])))
	}
}

// TestIdenticalUpdateAppliedTwice updates a base trie in the same way twice.
// Hence, the forest should de-duplicate the resulting two version of the identical trie
// without an error.
func TestIdenticalUpdateAppliedTwice(t *testing.T) {
	pathByteSize := 2 // path size of 16 bits
	dir, err := ioutil.TempDir("", "test-mtrie-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	forest, err := NewForest(pathByteSize, dir, 5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	p1 := pathByUint8s([]uint8{uint8(53), uint8(74)}, pathByteSize)
	v1 := payloadBySlices([]byte{'A'}, []byte{'A'})
	p2 := pathByUint8s([]uint8{uint8(116), uint8(129)}, pathByteSize)
	v2 := payloadBySlices([]byte{'B'}, []byte{'B'})
	paths := []ledger.Path{p1, p2}
	payloads := []*ledger.Payload{v1, v2}
	update := &ledger.TrieUpdate{RootHash: forest.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
	baseRoot, err := forest.Update(update)
	require.NoError(t, err)

	p3 := pathByUint8s([]uint8{uint8(116), uint8(22)}, pathByteSize)
	v3 := payloadBySlices([]byte{'D'}, []byte{'D'})

	update = &ledger.TrieUpdate{RootHash: baseRoot, Paths: []ledger.Path{p3}, Payloads: []*ledger.Payload{v3}}
	updatedRootA, err := forest.Update(update)
	require.NoError(t, err)
	updatedRootB, err := forest.Update(update)
	require.NoError(t, err)
	require.Equal(t, updatedRootA, updatedRootB)

	paths = []ledger.Path{p1, p2, p3}
	payloads = []*ledger.Payload{v1, v2, v3}
	read := &ledger.TrieRead{RootHash: updatedRootA, Paths: paths}
	retPayloadsA, err := forest.Read(read)
	require.NoError(t, err)
	for i := range paths {
		require.True(t, bytes.Equal(encoding.EncodePayload(retPayloadsA[i]), encoding.EncodePayload(payloads[i])))
	}

	read = &ledger.TrieRead{RootHash: updatedRootB, Paths: paths}
	retPayloadsB, err := forest.Read(read)
	require.NoError(t, err)
	for i := range paths {
		require.True(t, bytes.Equal(encoding.EncodePayload(retPayloadsB[i]), encoding.EncodePayload(payloads[i])))
	}
}

// TestRandomUpdateReadProof repeats a sequence of actions update, read and proof random paths
// this simulates the common patern of actions on flow
func TestRandomUpdateReadProof(t *testing.T) {
	pathByteSize := 2 // path size of 16 bits
	minPayloadByteSize := 2
	maxPayloadByteSize := 10
	rep := 10
	maxNumPathsPerStep := 10
	rand.Seed(time.Now().UnixNano())
	dir, err := ioutil.TempDir("", "test-mtrie-")
	require.NoError(t, err)
	defer os.RemoveAll(dir) // clean up

	forest, err := NewForest(pathByteSize, dir, 5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	activeRoot := forest.GetEmptyRootHash()
	require.NoError(t, err)
	latestPayloadByPath := make(map[string]*ledger.Payload) // map store

	for e := 0; e < rep; e++ {
		paths := utils.RandomPathsRandLen(maxNumPathsPerStep, pathByteSize)
		payloads := utils.RandomPayloads(len(paths), minPayloadByteSize, maxPayloadByteSize)

		// update map store with key values
		// we use this at the end of each step to check all existing keys
		for i, p := range paths {
			latestPayloadByPath[string(p)] = payloads[i]
		}

		// test reading for non-existing keys
		nonExistingPaths := make([]ledger.Path, 0)
		for _, p := range paths {
			if _, ok := latestPayloadByPath[string(p)]; !ok {
				nonExistingPaths = append(nonExistingPaths, p)
			}
		}
		read := &ledger.TrieRead{RootHash: activeRoot, Paths: nonExistingPaths}
		retPayloads, err := forest.Read(read)
		require.NoError(t, err, "error reading - non existing paths")
		for _, p := range retPayloads {
			require.True(t, p.IsEmpty())
		}

		// test update
		update := &ledger.TrieUpdate{RootHash: activeRoot, Paths: paths, Payloads: payloads}
		activeRoot, err = forest.Update(update)
		require.NoError(t, err, "error updating")

		// test read
		read = &ledger.TrieRead{RootHash: activeRoot, Paths: paths}
		retPayloads, err = forest.Read(read)
		require.NoError(t, err, "error reading")
		for i := range payloads {
			require.True(t, bytes.Equal(encoding.EncodePayload(retPayloads[i]), encoding.EncodePayload(payloads[i])))
		}

		// test proof (mix of existing and non existing keys)
		proofPaths := make([]ledger.Path, 0)
		proofPaths = append(proofPaths, paths...)
		proofPaths = append(proofPaths, nonExistingPaths...)

		read = &ledger.TrieRead{RootHash: activeRoot, Paths: proofPaths}
		batchProof, err := forest.Proofs(read)
		require.NoError(t, err, "error generating proofs")
		require.True(t, common.VerifyTrieBatchProof(batchProof, activeRoot))

		psmt, err := ptrie.NewPSMT(activeRoot, pathByteSize, batchProof)
		require.NoError(t, err, "error building partial trie")
		require.True(t, bytes.Equal(psmt.RootHash(), activeRoot))

		// check payloads for all existing paths
		allPaths := make([]ledger.Path, 0, len(latestPayloadByPath))
		allPayloads := make([]*ledger.Payload, 0, len(latestPayloadByPath))
		for p, v := range latestPayloadByPath {
			allPaths = append(allPaths, ledger.Path(p))
			allPayloads = append(allPayloads, v)
		}

		read = &ledger.TrieRead{RootHash: activeRoot, Paths: allPaths}
		retPayloads, err = forest.Read(read)
		require.NoError(t, err)
		for i, v := range allPayloads {
			require.True(t, bytes.Equal(encoding.EncodePayload(v), encoding.EncodePayload(retPayloads[i])))
		}
	}
}

// TestProofGenerationInclusion tests that inclusion proofs generated by a Trie pass verification
func TestProofGenerationInclusion(t *testing.T) {
	pathByteSize := 2 // path size of 16 bits
	dir, err := ioutil.TempDir("", "test-mtrie-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	metricsCollector := &metrics.NoopCollector{}
	forest, err := NewForest(pathByteSize, dir, 5, metricsCollector, nil)
	require.NoError(t, err)
	emptyRoot := forest.GetEmptyRootHash()

	p1 := pathByUint8s([]uint8{uint8(1), uint8(74)}, pathByteSize)
	v1 := payloadBySlices([]byte{'A'}, []byte{'A'})
	p2 := pathByUint8s([]uint8{uint8(2), uint8(74)}, pathByteSize)
	v2 := payloadBySlices([]byte{'B'}, []byte{'B'})
	p3 := pathByUint8s([]uint8{uint8(130), uint8(74)}, pathByteSize)
	v3 := payloadBySlices([]byte{'C'}, []byte{'C'})
	p4 := pathByUint8s([]uint8{uint8(131), uint8(74)}, pathByteSize)
	v4 := payloadBySlices([]byte{'D'}, []byte{'D'})
	paths := []ledger.Path{p1, p2, p3, p4}
	payloads := []*ledger.Payload{v1, v2, v3, v4}

	update := &ledger.TrieUpdate{RootHash: emptyRoot, Paths: paths, Payloads: payloads}
	updatedRoot, err := forest.Update(update)
	require.NoError(t, err)
	read := &ledger.TrieRead{RootHash: updatedRoot, Paths: paths}
	proof, err := forest.Proofs(read)

	require.NoError(t, err)
	require.True(t, common.VerifyTrieBatchProof(proof, ledger.State(updatedRoot)))
}

func payloadBySlices(keydata []byte, valuedata []byte) *ledger.Payload {
	key := ledger.Key{KeyParts: []ledger.KeyPart{ledger.KeyPart{Type: 0, Value: keydata}}}
	value := ledger.Value(valuedata)
	return &ledger.Payload{Key: key, Value: value}
}

func pathByUint8s(inputs []uint8, pathByteSize int) ledger.Path {
	b := make([]byte, pathByteSize)
	copy(b, inputs)
	return ledger.Path([]byte(b))
}
