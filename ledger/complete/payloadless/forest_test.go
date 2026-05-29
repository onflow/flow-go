package payloadless

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/module/metrics"
)

// TestTrieOperations tests adding removing and retrieving Trie from Forest
func TestTrieOperations(t *testing.T) {

	forest, err := NewForest(5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	// Make new Trie (independently of MForest):
	nt := NewEmptyMTrie()
	p1 := pathByUint8s([]uint8{uint8(53), uint8(74)})
	v1 := payloadBySlices([]byte{'A'}, []byte{'A'})

	updatedTrie, _, err := NewTrieWithUpdatedRegisters(nt, []ledger.Path{p1}, [][]byte{[]byte(v1.Value())}, true)
	require.NoError(t, err)

	// Add trie
	err = forest.AddTrie(updatedTrie)
	require.NoError(t, err)

	// Get trie
	retnt, err := forest.GetTrie(updatedTrie.RootHash())
	require.NoError(t, err)
	require.Equal(t, retnt.RootHash(), updatedTrie.RootHash())
	require.Equal(t, 2, forest.Size())
}

// TestTrieUpdate updates the empty trie with some values and verifies that the
// written leaf hashes can be retrieved from the updated trie.
func TestTrieUpdate(t *testing.T) {

	metricsCollector := &metrics.NoopCollector{}
	forest, err := NewForest(5, metricsCollector, nil)
	require.NoError(t, err)
	rootHash := forest.GetEmptyRootHash()

	p1 := pathByUint8s([]uint8{uint8(53), uint8(74)})
	v1 := payloadBySlices([]byte{'A'}, []byte{'A'})

	paths := []ledger.Path{p1}
	payloads := []*ledger.Payload{v1}
	update := &ledger.TrieUpdate{RootHash: rootHash, Paths: paths, Payloads: payloads}
	updatedRoot, err := forest.Update(update)
	require.NoError(t, err)

	read := &ledger.TrieRead{RootHash: updatedRoot, Paths: paths}
	retLeafHashes, err := forest.ReadLeafHashes(read)
	require.NoError(t, err)
	requireLeafHashesMatch(t, paths, payloads, retLeafHashes)
}

// TestLeftEmptyInsert tests inserting a new value into an empty sub-trie:
//  1. we first construct a baseTrie holding a couple of values on the right branch [~]
//  2. we update a previously non-existent register on the left branch (X)
//
// We verify that leaf hashes for _all_ paths in the updated Trie have correct values
func TestLeftEmptyInsert(t *testing.T) {
	//////////////////////
	//     insert X     //
	//       ()         //
	//      /  \        //
	//    (X)  [~]      //
	//////////////////////

	forest, err := NewForest(5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	// path: 1000...
	p1 := pathByUint8s([]uint8{uint8(129), uint8(1)})
	v1 := payloadBySlices([]byte{'A'}, []byte{'A'})

	// path: 1100...
	p2 := pathByUint8s([]uint8{uint8(193), uint8(1)})
	v2 := payloadBySlices([]byte{'B'}, []byte{'B'})

	paths := []ledger.Path{p1, p2}
	payloads := []*ledger.Payload{v1, v2}
	update := &ledger.TrieUpdate{RootHash: forest.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
	baseRoot, err := forest.Update(update)
	require.NoError(t, err)

	baseTrie, err := forest.GetTrie(baseRoot)
	require.NoError(t, err)
	require.Equal(t, uint64(2), baseTrie.AllocatedRegCount())

	p3 := pathByUint8s([]uint8{uint8(1), uint8(1)})
	v3 := payloadBySlices([]byte{'C'}, []byte{'C'})

	paths = []ledger.Path{p3}
	payloads = []*ledger.Payload{v3}
	update = &ledger.TrieUpdate{RootHash: baseTrie.RootHash(), Paths: paths, Payloads: payloads}
	updatedRoot, err := forest.Update(update)
	require.NoError(t, err)

	updatedTrie, err := forest.GetTrie(updatedRoot)
	require.NoError(t, err)
	require.Equal(t, uint64(3), updatedTrie.AllocatedRegCount())
	paths = []ledger.Path{p1, p2, p3}
	payloads = []*ledger.Payload{v1, v2, v3}
	read := &ledger.TrieRead{RootHash: updatedRoot, Paths: paths}
	retLeafHashes, err := forest.ReadLeafHashes(read)
	require.NoError(t, err)
	requireLeafHashesMatch(t, paths, payloads, retLeafHashes)
}

// TestRightEmptyInsert tests inserting a new value into an empty sub-trie:
//  1. we first construct a baseTrie holding a couple of values on the left branch [~]
//  2. we update a previously non-existent register on the right branch (X)
//
// We verify that leaf hashes for _all_ paths in the updated Trie have correct values
func TestRightEmptyInsert(t *testing.T) {
	///////////////////////
	//     insert X      //
	//       ()          //
	//      /  \         //
	//    [~]  (X)       //
	///////////////////////
	forest, err := NewForest(5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	// path: 0000...
	p1 := pathByUint8s([]uint8{uint8(1), uint8(1)})
	v1 := payloadBySlices([]byte{'A'}, []byte{'A'})

	// path: 0100...
	p2 := pathByUint8s([]uint8{uint8(64), uint8(1)})
	v2 := payloadBySlices([]byte{'B'}, []byte{'B'})

	paths := []ledger.Path{p1, p2}
	payloads := []*ledger.Payload{v1, v2}
	update := &ledger.TrieUpdate{RootHash: forest.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
	baseRoot, err := forest.Update(update)
	require.NoError(t, err)

	baseTrie, err := forest.GetTrie(baseRoot)
	require.NoError(t, err)
	require.Equal(t, uint64(2), baseTrie.AllocatedRegCount())

	// path: 1000...
	p3 := pathByUint8s([]uint8{uint8(129), uint8(1)})
	v3 := payloadBySlices([]byte{'C'}, []byte{'C'})

	paths = []ledger.Path{p3}
	payloads = []*ledger.Payload{v3}
	update = &ledger.TrieUpdate{RootHash: baseTrie.RootHash(), Paths: paths, Payloads: payloads}
	updatedRoot, err := forest.Update(update)
	require.NoError(t, err)

	updatedTrie, err := forest.GetTrie(updatedRoot)
	require.NoError(t, err)
	require.Equal(t, uint64(3), updatedTrie.AllocatedRegCount())

	paths = []ledger.Path{p1, p2, p3}
	payloads = []*ledger.Payload{v1, v2, v3}
	read := &ledger.TrieRead{RootHash: updatedRoot, Paths: paths}
	retLeafHashes, err := forest.ReadLeafHashes(read)
	require.NoError(t, err)
	requireLeafHashesMatch(t, paths, payloads, retLeafHashes)
}

// TestExpansionInsert tests inserting a new value into a populated sub-trie, where a
// leaf (holding a single value) would be replaced by an expanded sub-trie holding multiple value
//  1. we first construct a baseTrie holding a couple of values on the right branch [~]
//  2. we update a previously non-existent register on the right branch turning [~] to [~']
//
// We verify that leaf hashes for _all_ paths in the updated Trie are correct
func TestExpansionInsert(t *testing.T) {
	////////////////////////
	// modify [~] -> [~'] //
	//       ()           //
	//      /  \          //
	//         [~]        //
	////////////////////////

	forest, err := NewForest(5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	// path: 100000...
	p1 := pathByUint8s([]uint8{uint8(129), uint8(1)})
	v1 := payloadBySlices([]byte{'A'}, []byte{'A'})

	paths := []ledger.Path{p1}
	payloads := []*ledger.Payload{v1}
	update := &ledger.TrieUpdate{RootHash: forest.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
	baseRoot, err := forest.Update(update)
	require.NoError(t, err)

	baseTrie, err := forest.GetTrie(baseRoot)
	require.NoError(t, err)
	require.Equal(t, uint64(1), baseTrie.AllocatedRegCount())

	// path: 1000001...
	p2 := pathByUint8s([]uint8{uint8(130), uint8(1)})
	v2 := payloadBySlices([]byte{'B'}, []byte{'B'})

	paths = []ledger.Path{p2}
	payloads = []*ledger.Payload{v2}
	update = &ledger.TrieUpdate{RootHash: baseTrie.RootHash(), Paths: paths, Payloads: payloads}
	updatedRoot, err := forest.Update(update)
	require.NoError(t, err)

	updatedTrie, err := forest.GetTrie(updatedRoot)
	require.NoError(t, err)
	require.Equal(t, uint64(2), updatedTrie.AllocatedRegCount())

	paths = []ledger.Path{p1, p2}
	payloads = []*ledger.Payload{v1, v2}
	read := &ledger.TrieRead{RootHash: updatedRoot, Paths: paths}
	retLeafHashes, err := forest.ReadLeafHashes(read)
	require.NoError(t, err)
	requireLeafHashesMatch(t, paths, payloads, retLeafHashes)
}

// TestFullHouseInsert tests inserting a new value into a populated sub-trie, where a
// leaf's value is overridden _and_ further values are added which all fall into a subtree that
// replaces the leaf:
//  1. we first construct a baseTrie holding a couple of values on the right branch [~]
//  2. we update a previously non-existent register on the right branch turning [~] to [~']
//
// We verify that leaf hashes for _all_ paths in the updated Trie are correct
func TestFullHouseInsert(t *testing.T) {
	///////////////////////
	//   insert ~1<X<~2  //
	//       ()          //
	//      /  \         //
	//    [~1]  [~2]     //
	///////////////////////

	forest, err := NewForest(5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	// paths p0 forms [~1]; p1 and p2 form [~2]
	// path: 0100...
	p0 := pathByUint8s([]uint8{uint8(64), uint8(1)})
	v0 := payloadBySlices([]byte{'0'}, []byte{'0'})
	// path: 1000...
	p1 := pathByUint8s([]uint8{uint8(129), uint8(1)})
	v1 := payloadBySlices([]byte{'A'}, []byte{'A'})

	// path: 1100...
	p2 := pathByUint8s([]uint8{uint8(193), uint8(1)})
	v2 := payloadBySlices([]byte{'B'}, []byte{'B'})

	paths := []ledger.Path{p0, p1, p2}
	payloads := []*ledger.Payload{v0, v1, v2}
	update := &ledger.TrieUpdate{RootHash: forest.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
	baseRoot, err := forest.Update(update)
	require.NoError(t, err)

	baseTrie, err := forest.GetTrie(baseRoot)
	require.NoError(t, err)
	require.Equal(t, uint64(3), baseTrie.AllocatedRegCount())

	// we update value for path p1 and in addition add p3 that has the same prefix `10` as p1
	v1 = payloadBySlices([]byte{'X'}, []byte{'X'})

	// path: 1010...
	p3 := pathByUint8s([]uint8{uint8(160), uint8(1)})
	v3 := payloadBySlices([]byte{'C'}, []byte{'C'})

	paths = []ledger.Path{p1, p3}
	payloads = []*ledger.Payload{v1, v3}
	update = &ledger.TrieUpdate{RootHash: baseTrie.RootHash(), Paths: paths, Payloads: payloads}
	updatedRoot, err := forest.Update(update)
	require.NoError(t, err)

	updatedTrie, err := forest.GetTrie(updatedRoot)
	require.NoError(t, err)
	require.Equal(t, uint64(4), updatedTrie.AllocatedRegCount())

	paths = []ledger.Path{p1, p2, p3}
	payloads = []*ledger.Payload{v1, v2, v3}
	read := &ledger.TrieRead{RootHash: updatedRoot, Paths: paths}
	retLeafHashes, err := forest.ReadLeafHashes(read)
	require.NoError(t, err)
	requireLeafHashesMatch(t, paths, payloads, retLeafHashes)
}

// TestLeafInsert inserts two keys, which only differ in their last bit.
// I.e. the trie needs to be expanded to its hull depth
// We verify that leaf hashes for _all_ paths in the updated Trie are correct
func TestLeafInsert(t *testing.T) {
	///////////////////////
	//   insert 1, 2     //
	//       ()          //
	//      /  \         //
	//     ()   ...      //
	//          /  \     //
	//         ()  ()    //
	///////////////////////
	forest, err := NewForest(5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	// path: 000...0000000100000000
	p1 := testutils.PathByUint16LeftPadded(256)
	v1 := payloadBySlices([]byte{'A'}, []byte{'A'})

	// path: 000...0000000100000001
	p2 := testutils.PathByUint16LeftPadded(257)
	v2 := payloadBySlices([]byte{'B'}, []byte{'B'})

	paths := []ledger.Path{p1, p2}
	payloads := []*ledger.Payload{v1, v2}
	update := &ledger.TrieUpdate{RootHash: forest.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
	updatedRoot, err := forest.Update(update)
	require.NoError(t, err)

	updatedTrie, err := forest.GetTrie(updatedRoot)
	require.NoError(t, err)
	require.Equal(t, uint64(2), updatedTrie.AllocatedRegCount())

	read := &ledger.TrieRead{RootHash: updatedRoot, Paths: paths}
	retLeafHashes, err := forest.ReadLeafHashes(read)
	require.NoError(t, err)
	requireLeafHashesMatch(t, paths, payloads, retLeafHashes)
}

// TestOverrideValue overrides an existing value in the trie (without any expansion)
// We verify that leaf hashes for _all_ paths in the updated Trie are correct
func TestOverrideValue(t *testing.T) {
	forest, err := NewForest(5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	// path: 1000...
	p1 := pathByUint8s([]uint8{uint8(53), uint8(74)})
	v1 := payloadBySlices([]byte{'A'}, []byte{'A'})

	// path: 0111...
	p2 := pathByUint8s([]uint8{uint8(116), uint8(129)})
	v2 := payloadBySlices([]byte{'B'}, []byte{'B'})

	paths := []ledger.Path{p1, p2}
	payloads := []*ledger.Payload{v1, v2}
	update := &ledger.TrieUpdate{RootHash: forest.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
	baseRoot, err := forest.Update(update)
	require.NoError(t, err)

	// path: 1000...
	p3 := pathByUint8s([]uint8{uint8(53), uint8(74)})
	v3 := payloadBySlices([]byte{'C'}, []byte{'C'})

	paths = []ledger.Path{p3}
	payloads = []*ledger.Payload{v3}
	update = &ledger.TrieUpdate{RootHash: baseRoot, Paths: paths, Payloads: payloads}
	updatedRoot, err := forest.Update(update)
	require.NoError(t, err)

	read := &ledger.TrieRead{RootHash: updatedRoot, Paths: paths}
	retLeafHashes, err := forest.ReadLeafHashes(read)
	require.NoError(t, err)
	requireLeafHashesMatch(t, paths, payloads, retLeafHashes)
}

// TestDuplicateOverride tests behaviour when the updates contain two different payloads for the
// same path. I.e. we update with (p0, v0) and (p0, v1)
// We expect that the _last_ written value is persisted in the Trie
func TestDuplicateOverride(t *testing.T) {

	forest, err := NewForest(5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	// path: 1000...
	p0 := pathByUint8s([]uint8{uint8(53), uint8(74)})
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
	retLeafHashes, err := forest.ReadLeafHashes(read)
	require.NoError(t, err)
	requireLeafHashesMatch(t, paths, []*ledger.Payload{v2}, retLeafHashes)

}

// TestReadSafety checks that the slice returned by ReadLeafHashes is freshly allocated
// per call, so modifying entries in one returned slice cannot affect subsequent reads.
func TestReadSafety(t *testing.T) {
	forest, err := NewForest(5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	// path: 1000...
	p0 := pathByUint8s([]uint8{uint8(53), uint8(74)})
	v0 := payloadBySlices([]byte{'A'}, []byte{'A'})
	paths := []ledger.Path{p0}
	payloads := []*ledger.Payload{v0}
	update := &ledger.TrieUpdate{RootHash: forest.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
	baseRoot, err := forest.Update(update)
	require.NoError(t, err)

	read := &ledger.TrieRead{RootHash: baseRoot, Paths: paths}
	data, err := forest.ReadLeafHashes(read)
	require.NoError(t, err)
	require.Len(t, data, 1)
	expected := hash.HashLeaf(hash.Hash(p0), []byte(v0.Value()))
	require.NotNil(t, data[0])
	require.Equal(t, expected, *data[0])

	// modify returned slice element
	data[0] = nil

	// read again, should not be affected
	data2, err := forest.ReadLeafHashes(read)
	require.NoError(t, err)
	require.Len(t, data2, 1)
	require.NotNil(t, data2[0])
	require.Equal(t, expected, *data2[0])
}

// TestReadOrder tests that leaf hashes from reading a trie are delivered in the order as specified by the paths
func TestReadOrder(t *testing.T) {

	forest, err := NewForest(5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	p1 := pathByUint8s([]uint8{uint8(116), uint8(74)})
	v1 := payloadBySlices([]byte{'A'}, []byte{'A'})

	p2 := pathByUint8s([]uint8{uint8(53), uint8(129)})
	v2 := payloadBySlices([]byte{'B'}, []byte{'B'})

	paths := []ledger.Path{p1, p2}
	payloads := []*ledger.Payload{v1, v2}
	update := &ledger.TrieUpdate{RootHash: forest.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
	testRoot, err := forest.Update(update)
	require.NoError(t, err)

	read := &ledger.TrieRead{RootHash: testRoot, Paths: []ledger.Path{p1, p2}}
	retLeafHashes, err := forest.ReadLeafHashes(read)
	require.NoError(t, err)
	require.Equal(t, len(read.Paths), len(retLeafHashes))
	requireLeafHashesMatch(t, []ledger.Path{p1, p2}, []*ledger.Payload{v1, v2}, retLeafHashes)

	read = &ledger.TrieRead{RootHash: testRoot, Paths: []ledger.Path{p2, p1}}
	retLeafHashes, err = forest.ReadLeafHashes(read)
	require.NoError(t, err)
	require.Equal(t, len(read.Paths), len(retLeafHashes))
	requireLeafHashesMatch(t, []ledger.Path{p2, p1}, []*ledger.Payload{v2, v1}, retLeafHashes)
}

// TestMixRead tests reading a mixture of set and unset registers.
// We expect a nil leaf hash to be returned for unset registers.
func TestMixRead(t *testing.T) {
	forest, err := NewForest(5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	// path: 01111101...
	p1 := pathByUint8s([]uint8{uint8(125), uint8(23)})
	v1 := payloadBySlices([]byte{'A'}, []byte{'A'})

	// path: 10110010...
	p2 := pathByUint8s([]uint8{uint8(178), uint8(152)})
	v2 := payloadBySlices([]byte{'B'}, []byte{'B'})

	paths := []ledger.Path{p1, p2}
	payloads := []*ledger.Payload{v1, v2}

	update := &ledger.TrieUpdate{RootHash: forest.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
	baseRoot, err := forest.Update(update)
	require.NoError(t, err)

	// path: 01101110...
	p3 := pathByUint8s([]uint8{uint8(110), uint8(48)})
	v3 := ledger.EmptyPayload()

	// path: 00010111...
	p4 := pathByUint8s([]uint8{uint8(23), uint8(82)})
	v4 := ledger.EmptyPayload()

	readPaths := []ledger.Path{p1, p2, p3, p4}
	expectedPayloads := []*ledger.Payload{v1, v2, v3, v4}

	read := &ledger.TrieRead{RootHash: baseRoot, Paths: readPaths}
	retLeafHashes, err := forest.ReadLeafHashes(read)
	require.NoError(t, err)
	requireLeafHashesMatch(t, readPaths, expectedPayloads, retLeafHashes)
}

// TestReadWithDuplicatedKeys reads the leaf hashes for two keys, where both keys have the same value.
// We expect that we receive the respective leaf hash twice in the return.
func TestReadWithDuplicatedKeys(t *testing.T) {
	forest, err := NewForest(5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	p1 := pathByUint8s([]uint8{uint8(53), uint8(74)})
	v1 := payloadBySlices([]byte{'A'}, []byte{'A'})
	p2 := pathByUint8s([]uint8{uint8(116), uint8(129)})
	v2 := payloadBySlices([]byte{'B'}, []byte{'B'})
	p3 := pathByUint8s([]uint8{uint8(53), uint8(74)})

	paths := []ledger.Path{p1, p2}
	payloads := []*ledger.Payload{v1, v2}
	update := &ledger.TrieUpdate{RootHash: forest.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
	updatedRoot, err := forest.Update(update)
	require.NoError(t, err)

	paths = []ledger.Path{p1, p2, p3}
	expectedPayloads := []*ledger.Payload{v1, v2, v1}
	read := &ledger.TrieRead{RootHash: updatedRoot, Paths: paths}
	retLeafHashes, err := forest.ReadLeafHashes(read)
	require.NoError(t, err)
	require.Equal(t, len(read.Paths), len(retLeafHashes))
	requireLeafHashesMatch(t, paths, expectedPayloads, retLeafHashes)
}

// TestReadNonExistingPath tests reading an unset path.
func TestReadNonExistingPath(t *testing.T) {
	forest, err := NewForest(5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	p1 := pathByUint8s([]uint8{uint8(53), uint8(74)})
	v1 := payloadBySlices([]byte{'A'}, []byte{'A'})
	paths := []ledger.Path{p1}
	payloads := []*ledger.Payload{v1}

	update := &ledger.TrieUpdate{RootHash: forest.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
	updatedRoot, err := forest.Update(update)
	require.NoError(t, err)

	p2 := pathByUint8s([]uint8{uint8(116), uint8(129)})
	read := &ledger.TrieRead{RootHash: updatedRoot, Paths: []ledger.Path{p2}}
	retLeafHashes, err := forest.ReadLeafHashes(read)
	require.NoError(t, err)
	require.Nil(t, retLeafHashes[0])
}

// TestReadSingleLeafHash tests reading a single leaf hash for set/unset registers.
func TestReadSingleLeafHash(t *testing.T) {
	forest, err := NewForest(5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	// path: 01111101...
	path1 := pathByUint8s([]uint8{uint8(125), uint8(23)})
	payload1 := payloadBySlices([]byte{'A'}, []byte{'A'})

	// path: 10110010...
	path2 := pathByUint8s([]uint8{uint8(178), uint8(152)})
	payload2 := payloadBySlices([]byte{'B'}, []byte{'B'})

	paths := []ledger.Path{path1, path2}
	payloads := []*ledger.Payload{payload1, payload2}

	update := &ledger.TrieUpdate{RootHash: forest.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
	baseRoot, err := forest.Update(update)
	require.NoError(t, err)

	// path: 01101110...
	path3 := pathByUint8s([]uint8{uint8(110), uint8(48)})
	payload3 := ledger.EmptyPayload()

	// path: 00010111...
	path4 := pathByUint8s([]uint8{uint8(23), uint8(82)})
	payload4 := ledger.EmptyPayload()

	expectedPayloads := map[ledger.Path]*ledger.Payload{
		path1: payload1,
		path2: payload2,
		path3: payload3,
		path4: payload4,
	}

	// Batch read one leaf hash at a time (less efficient)
	for path, payload := range expectedPayloads {
		read := &ledger.TrieRead{RootHash: baseRoot, Paths: []ledger.Path{path}}
		retLeafHashes, err := forest.ReadLeafHashes(read)
		require.NoError(t, err)
		require.Equal(t, 1, len(retLeafHashes))
		if payload.IsEmpty() {
			require.Nil(t, retLeafHashes[0])
		} else {
			expected := hash.HashLeaf(hash.Hash(path), []byte(payload.Value()))
			require.NotNil(t, retLeafHashes[0])
			require.Equal(t, expected, *retLeafHashes[0])
		}
	}

	// Read single leaf hash
	for path, payload := range expectedPayloads {
		read := &ledger.TrieReadSingleValue{RootHash: baseRoot, Path: path}
		retLeafHash, err := forest.ReadSingleLeafHash(read)
		require.NoError(t, err)
		if payload.IsEmpty() {
			require.Nil(t, retLeafHash)
		} else {
			expected := hash.HashLeaf(hash.Hash(path), []byte(payload.Value()))
			require.NotNil(t, retLeafHash)
			require.Equal(t, expected, *retLeafHash)
		}
	}
}

// TestForkingUpdates updates a base trie in two different ways. We expect
// that for each update, a new trie is added to the forest preserving the
// updated values independently of the other update.
func TestForkingUpdates(t *testing.T) {
	forest, err := NewForest(5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	p1 := pathByUint8s([]uint8{uint8(53), uint8(74)})
	v1 := payloadBySlices([]byte{'A'}, []byte{'A'})
	p2 := pathByUint8s([]uint8{uint8(116), uint8(129)})
	v2 := payloadBySlices([]byte{'B'}, []byte{'B'})
	paths := []ledger.Path{p1, p2}
	payloads := []*ledger.Payload{v1, v2}
	update := &ledger.TrieUpdate{RootHash: forest.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
	baseRoot, err := forest.Update(update)
	require.NoError(t, err)

	// update baseTrie -> updatedTrieA
	v1a := payloadBySlices([]byte{'C'}, []byte{'C'})
	p3a := pathByUint8s([]uint8{uint8(116), uint8(22)})
	v3a := payloadBySlices([]byte{'D'}, []byte{'D'})
	pathsA := []ledger.Path{p1, p3a}
	payloadsA := []*ledger.Payload{v1a, v3a}
	updateA := &ledger.TrieUpdate{RootHash: baseRoot, Paths: pathsA, Payloads: payloadsA}
	updatedRootA, err := forest.Update(updateA)
	require.NoError(t, err)

	// update baseTrie -> updatedTrieB
	v1b := payloadBySlices([]byte{'E'}, []byte{'E'})
	p3b := pathByUint8s([]uint8{uint8(116), uint8(22)})
	v3b := payloadBySlices([]byte{'F'}, []byte{'F'})
	pathsB := []ledger.Path{p1, p3b}
	payloadsB := []*ledger.Payload{v1b, v3b}
	updateB := &ledger.TrieUpdate{RootHash: baseRoot, Paths: pathsB, Payloads: payloadsB}
	updatedRootB, err := forest.Update(updateB)
	require.NoError(t, err)

	// Verify leaf hashes are preserved
	read := &ledger.TrieRead{RootHash: baseRoot, Paths: paths}
	retLeafHashes, err := forest.ReadLeafHashes(read)
	require.NoError(t, err)
	requireLeafHashesMatch(t, paths, payloads, retLeafHashes)

	readA := &ledger.TrieRead{RootHash: updatedRootA, Paths: pathsA}
	retLeafHashes, err = forest.ReadLeafHashes(readA)
	require.NoError(t, err)
	requireLeafHashesMatch(t, pathsA, payloadsA, retLeafHashes)

	readB := &ledger.TrieRead{RootHash: updatedRootB, Paths: pathsB}
	retLeafHashes, err = forest.ReadLeafHashes(readB)
	require.NoError(t, err)
	requireLeafHashesMatch(t, pathsB, payloadsB, retLeafHashes)
}

// TestIdenticalUpdateAppliedTwice updates a base trie in the same way twice.
// Hence, the forest should de-duplicate the resulting two version of the identical trie
// without an error.
func TestIdenticalUpdateAppliedTwice(t *testing.T) {
	forest, err := NewForest(5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	p1 := pathByUint8s([]uint8{uint8(53), uint8(74)})
	v1 := payloadBySlices([]byte{'A'}, []byte{'A'})
	p2 := pathByUint8s([]uint8{uint8(116), uint8(129)})
	v2 := payloadBySlices([]byte{'B'}, []byte{'B'})
	paths := []ledger.Path{p1, p2}
	payloads := []*ledger.Payload{v1, v2}
	update := &ledger.TrieUpdate{RootHash: forest.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
	baseRoot, err := forest.Update(update)
	require.NoError(t, err)

	p3 := pathByUint8s([]uint8{uint8(116), uint8(22)})
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
	retLeafHashesA, err := forest.ReadLeafHashes(read)
	require.NoError(t, err)
	requireLeafHashesMatch(t, paths, payloads, retLeafHashesA)

	read = &ledger.TrieRead{RootHash: updatedRootB, Paths: paths}
	retLeafHashesB, err := forest.ReadLeafHashes(read)
	require.NoError(t, err)
	requireLeafHashesMatch(t, paths, payloads, retLeafHashesB)
}

func payloadBySlices(keydata []byte, valuedata []byte) *ledger.Payload {
	key := ledger.Key{KeyParts: []ledger.KeyPart{{Type: 0, Value: keydata}}}
	value := ledger.Value(valuedata)
	return ledger.NewPayload(key, value)
}

func pathByUint8s(inputs []uint8) ledger.Path {
	var b ledger.Path
	copy(b[:], inputs)
	return b
}

// requireLeafHashesMatch asserts that retLeafHashes[i] equals HashLeaf(paths[i], payloads[i].Value())
// for each non-empty payload, and is nil otherwise.
func requireLeafHashesMatch(t *testing.T, paths []ledger.Path, payloads []*ledger.Payload, retLeafHashes []*hash.Hash) {
	t.Helper()
	require.Equal(t, len(paths), len(retLeafHashes))
	for i := range paths {
		if payloads[i].IsEmpty() {
			require.Nil(t, retLeafHashes[i], "expected nil leaf hash at index %d", i)
			continue
		}
		require.NotNil(t, retLeafHashes[i], "expected non-nil leaf hash at index %d", i)
		expected := hash.HashLeaf(hash.Hash(paths[i]), []byte(payloads[i].Value()))
		require.Equal(t, expected, *retLeafHashes[i], "leaf hash mismatch at index %d", i)
	}
}

// TestHasPathsOrder tests returned existence flags are in the order as specified by the paths
func TestHasPathsOrder(t *testing.T) {

	forest, err := NewForest(5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	// path: 01111101...
	p1 := pathByUint8s([]uint8{uint8(125), uint8(23)})
	v1 := testutils.RandomPayload(1, 100)

	// path: 10110010...
	p2 := pathByUint8s([]uint8{uint8(178), uint8(152)})
	v2 := testutils.RandomPayload(1, 100)

	paths := []ledger.Path{p1, p2}
	payloads := []*ledger.Payload{v1, v2}
	update := &ledger.TrieUpdate{RootHash: forest.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
	baseRoot, err := forest.Update(update)
	require.NoError(t, err)

	// Get HasPaths for paths {p1, p2}
	read := &ledger.TrieRead{RootHash: baseRoot, Paths: []ledger.Path{p1, p2}}
	exists, err := forest.HasPaths(read)
	require.NoError(t, err)
	require.Equal(t, len(read.Paths), len(exists))
	require.True(t, exists[0])
	require.True(t, exists[1])

	// Get HasPaths for paths {p2, p1}
	read = &ledger.TrieRead{RootHash: baseRoot, Paths: []ledger.Path{p2, p1}}
	exists, err = forest.HasPaths(read)
	require.NoError(t, err)
	require.Equal(t, len(read.Paths), len(exists))
	require.True(t, exists[0])
	require.True(t, exists[1])
}

// TestMixHasPaths tests HasPaths for a mix of set and unset registers.
// We expect false to be returned for unset registers.
func TestMixHasPaths(t *testing.T) {
	forest, err := NewForest(5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	// path: 01111101...
	p1 := pathByUint8s([]uint8{uint8(125), uint8(23)})
	v1 := testutils.RandomPayload(1, 100)

	// path: 10110010...
	p2 := pathByUint8s([]uint8{uint8(178), uint8(152)})
	v2 := testutils.RandomPayload(1, 100)

	paths := []ledger.Path{p1, p2}
	payloads := []*ledger.Payload{v1, v2}
	update := &ledger.TrieUpdate{RootHash: forest.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
	baseRoot, err := forest.Update(update)
	require.NoError(t, err)

	// path: 01101110...
	p3 := pathByUint8s([]uint8{uint8(110), uint8(48)})

	// path: 00010111...
	p4 := pathByUint8s([]uint8{uint8(23), uint8(82)})

	readPaths := []ledger.Path{p1, p2, p3, p4}
	expected := []bool{true, true, false, false}

	read := &ledger.TrieRead{RootHash: baseRoot, Paths: readPaths}
	exists, err := forest.HasPaths(read)
	require.NoError(t, err)
	require.Equal(t, len(read.Paths), len(exists))
	for i := range read.Paths {
		require.Equal(t, expected[i], exists[i])
	}
}

// TestHasPathsWithDuplicatedKeys checks HasPaths for two keys, where both keys are equal.
// We expect to receive the same existence flag twice.
func TestHasPathsWithDuplicatedKeys(t *testing.T) {
	forest, err := NewForest(5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	// path: 01111101...
	p1 := pathByUint8s([]uint8{uint8(125), uint8(23)})
	v1 := testutils.RandomPayload(1, 100)

	// path: 10110010...
	p2 := pathByUint8s([]uint8{uint8(178), uint8(152)})
	v2 := testutils.RandomPayload(1, 100)

	// same path as p1
	p3 := pathByUint8s([]uint8{uint8(125), uint8(23)})

	paths := []ledger.Path{p1, p2}
	payloads := []*ledger.Payload{v1, v2}
	update := &ledger.TrieUpdate{RootHash: forest.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
	baseRoot, err := forest.Update(update)
	require.NoError(t, err)

	readPaths := []ledger.Path{p1, p2, p3}
	expected := []bool{true, true, true}

	read := &ledger.TrieRead{RootHash: baseRoot, Paths: readPaths}
	exists, err := forest.HasPaths(read)
	require.NoError(t, err)
	require.Equal(t, len(read.Paths), len(exists))
	for i := range read.Paths {
		require.Equal(t, expected[i], exists[i])
	}
}

func TestPurgeCacheExcept(t *testing.T) {
	forest, err := NewForest(5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	nt := NewEmptyMTrie()
	p1 := pathByUint8s([]uint8{uint8(53), uint8(74)})
	v1 := payloadBySlices([]byte{'A'}, []byte{'A'})

	updatedTrie1, _, err := NewTrieWithUpdatedRegisters(nt, []ledger.Path{p1}, [][]byte{[]byte(v1.Value())}, true)
	require.NoError(t, err)

	err = forest.AddTrie(updatedTrie1)
	require.NoError(t, err)

	p2 := pathByUint8s([]uint8{uint8(12), uint8(34)})
	v2 := payloadBySlices([]byte{'B'}, []byte{'B'})

	updatedTrie2, _, err := NewTrieWithUpdatedRegisters(nt, []ledger.Path{p2}, [][]byte{[]byte(v2.Value())}, true)
	require.NoError(t, err)

	err = forest.AddTrie(updatedTrie2)
	require.NoError(t, err)
	require.Equal(t, 3, forest.tries.Count())

	err = forest.PurgeCacheExcept(updatedTrie2.RootHash())
	require.NoError(t, err)
	require.Equal(t, 1, forest.tries.Count())

	ret, err := forest.GetTrie(updatedTrie2.RootHash())
	require.NoError(t, err)
	require.Equal(t, ret, updatedTrie2)

	_, err = forest.GetTrie(updatedTrie1.RootHash())
	require.Error(t, err)

	// test purge with non existing trie
	err = forest.PurgeCacheExcept(updatedTrie1.RootHash())
	require.Error(t, err)

	ret, err = forest.GetTrie(updatedTrie2.RootHash())
	require.NoError(t, err)
	require.Equal(t, ret, updatedTrie2)

	_, err = forest.GetTrie(updatedTrie1.RootHash())
	require.Error(t, err)

	// test purge when only a single target trie exist there
	err = forest.PurgeCacheExcept(updatedTrie2.RootHash())
	require.NoError(t, err)
	require.Equal(t, 1, forest.tries.Count())
}
