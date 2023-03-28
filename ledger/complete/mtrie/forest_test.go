package mtrie

import (
	"bytes"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	prf "github.com/onflow/flow-go/ledger/common/proof"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/ledger/partial/ptrie"
	"github.com/onflow/flow-go/module/metrics"
)

// TestTrieOperations tests adding removing and retrieving Trie from Forest
func TestTrieOperations(t *testing.T) {

	forest, err := NewForest(5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	// Make new Trie (independently of MForest):
	nt := trie.NewEmptyMTrie()
	p1 := pathByUint8s([]uint8{uint8(53), uint8(74)})
	v1 := payloadBySlices([]byte{'A'}, []byte{'A'})

	updatedTrie, _, err := trie.NewTrieWithUpdatedRegisters(nt, []ledger.Path{p1}, []ledger.Payload{*v1}, true)
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
// written values can be retrieved from the updated trie.
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
	retValues, err := forest.Read(read)
	require.NoError(t, err)
	require.Equal(t, retValues[0], payloads[0].Value())
}

// TestLeftEmptyInsert tests inserting a new value into an empty sub-trie:
//  1. we first construct a baseTrie holding a couple of values on the right branch [~]
//  2. we update a previously non-existent register on the left branch (X)
//
// We verify that values for _all_ paths in the updated Trie have correct payloads
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
	require.Equal(t, uint64(v1.Size()+v2.Size()), baseTrie.AllocatedRegSize())

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
	require.Equal(t, uint64(v1.Size()+v2.Size()+v3.Size()), updatedTrie.AllocatedRegSize())
	paths = []ledger.Path{p1, p2, p3}
	payloads = []*ledger.Payload{v1, v2, v3}
	read := &ledger.TrieRead{RootHash: updatedRoot, Paths: paths}
	retValues, err := forest.Read(read)
	require.NoError(t, err)
	for i := range paths {
		require.Equal(t, retValues[i], payloads[i].Value())
	}
}

// TestRightEmptyInsert tests inserting a new value into an empty sub-trie:
//  1. we first construct a baseTrie holding a couple of values on the left branch [~]
//  2. we update a previously non-existent register on the right branch (X)
//
// We verify that values for _all_ paths in the updated Trie have correct payloads
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
	require.Equal(t, uint64(v1.Size()+v2.Size()), baseTrie.AllocatedRegSize())

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
	require.Equal(t, uint64(v1.Size()+v2.Size()+v3.Size()), updatedTrie.AllocatedRegSize())

	paths = []ledger.Path{p1, p2, p3}
	payloads = []*ledger.Payload{v1, v2, v3}
	read := &ledger.TrieRead{RootHash: updatedRoot, Paths: paths}
	retValues, err := forest.Read(read)
	require.NoError(t, err)
	for i := range paths {
		require.Equal(t, retValues[i], payloads[i].Value())
	}
}

// TestExpansionInsert tests inserting a new value into a populated sub-trie, where a
// leaf (holding a single value) would be replaced by an expanded sub-trie holding multiple value
//  1. we first construct a baseTrie holding a couple of values on the right branch [~]
//  2. we update a previously non-existent register on the right branch turning [~] to [~']
//
// We verify that values for _all_ paths in the updated Trie have correct payloads
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
	require.Equal(t, uint64(v1.Size()), baseTrie.AllocatedRegSize())

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
	require.Equal(t, uint64(v1.Size()+v2.Size()), updatedTrie.AllocatedRegSize())

	paths = []ledger.Path{p1, p2}
	payloads = []*ledger.Payload{v1, v2}
	read := &ledger.TrieRead{RootHash: updatedRoot, Paths: paths}
	retValues, err := forest.Read(read)
	require.NoError(t, err)
	for i := range paths {
		require.Equal(t, retValues[i], payloads[i].Value())
	}
}

// TestFullHouseInsert tests inserting a new value into a populated sub-trie, where a
// leaf's value is overridden _and_ further values are added which all fall into a subtree that
// replaces the leaf:
//  1. we first construct a baseTrie holding a couple of values on the right branch [~]
//  2. we update a previously non-existent register on the right branch turning [~] to [~']
//
// We verify that values for _all_ paths in the updated Trie have correct payloads
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
	require.Equal(t, uint64(v0.Size()+v1.Size()+v2.Size()), baseTrie.AllocatedRegSize())

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
	require.Equal(t, uint64(v0.Size()+v1.Size()+v2.Size()+v3.Size()), updatedTrie.AllocatedRegSize())

	paths = []ledger.Path{p1, p2, p3}
	payloads = []*ledger.Payload{v1, v2, v3}
	read := &ledger.TrieRead{RootHash: updatedRoot, Paths: paths}
	retValues, err := forest.Read(read)
	require.NoError(t, err)
	for i := range paths {
		require.Equal(t, retValues[i], payloads[i].Value())
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
	require.Equal(t, uint64(v1.Size()+v2.Size()), updatedTrie.AllocatedRegSize())

	read := &ledger.TrieRead{RootHash: updatedRoot, Paths: paths}
	retValues, err := forest.Read(read)
	require.NoError(t, err)
	for i := range paths {
		require.Equal(t, retValues[i], payloads[i].Value())
	}
}

// TestOverrideValue overrides an existing value in the trie (without any expansion)
// We verify that values for _all_ paths in the updated Trie have correct payloads
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
	retValues, err := forest.Read(read)
	require.NoError(t, err)
	require.Equal(t, retValues[0], payloads[0].Value())

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
	retValues, err := forest.Read(read)
	require.NoError(t, err)
	require.Equal(t, retValues[0], v2.Value())

}

// TestReadSafety check if payload returned from a forest are safe against modification,
// ie. copy of the data is returned, instead of a slice
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
	data, err := forest.Read(read)
	require.NoError(t, err)

	require.Len(t, data, 1)
	require.Equal(t, v0.Value(), data[0])

	// modify returned slice
	data[0] = []byte("new value")

	// read again
	data2, err := forest.Read(read)
	require.NoError(t, err)
	require.Len(t, data2, 1)
	require.Equal(t, v0.Value(), data2[0])
}

// TestReadOrder tests that payloads from reading a trie are delivered in the order as specified by the paths
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
	retValues, err := forest.Read(read)
	require.NoError(t, err)
	require.Equal(t, len(read.Paths), len(retValues))
	require.Equal(t, retValues[0], payloads[0].Value())
	require.Equal(t, retValues[1], payloads[1].Value())

	read = &ledger.TrieRead{RootHash: testRoot, Paths: []ledger.Path{p2, p1}}
	retValues, err = forest.Read(read)
	require.NoError(t, err)
	require.Equal(t, len(read.Paths), len(retValues))
	require.Equal(t, retValues[1], payloads[0].Value())
	require.Equal(t, retValues[0], payloads[1].Value())
}

// TestMixRead tests reading a mixture of set and unset registers.
// We expect the default payload (nil) to be returned for unset registers.
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
	retValues, err := forest.Read(read)
	require.NoError(t, err)
	for i := range paths {
		require.Equal(t, retValues[i], expectedPayloads[i].Value())
	}
}

// TestReadWithDuplicatedKeys reads a the values for two keys, where both keys have the same value.
// We expect that we receive the respective value twice in the return.
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
	retValues, err := forest.Read(read)
	require.NoError(t, err)
	require.Equal(t, len(read.Paths), len(retValues))
	for i := range paths {
		require.Equal(t, retValues[i], expectedPayloads[i].Value())
	}
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
	retValues, err := forest.Read(read)
	require.NoError(t, err)
	require.Equal(t, 0, len(retValues[0]))
}

// TestReadSinglePayload tests reading a single payload of set/unset register.
func TestReadSinglePayload(t *testing.T) {
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

	expectedPayloads := make(map[ledger.Path]*ledger.Payload)
	expectedPayloads[path1] = payload1
	expectedPayloads[path2] = payload2
	expectedPayloads[path3] = payload3
	expectedPayloads[path4] = payload4

	// Batch read one payload at a time (less efficient)
	for path, payload := range expectedPayloads {
		read := &ledger.TrieRead{RootHash: baseRoot, Paths: []ledger.Path{path}}
		retValues, err := forest.Read(read)
		require.NoError(t, err)
		require.Equal(t, 1, len(retValues))
		if payload.IsEmpty() {
			require.Equal(t, 0, len(retValues[0]))
		} else {
			require.Equal(t, payload.Value(), retValues[0])
		}
	}

	// Read single value
	for path, payload := range expectedPayloads {
		read := &ledger.TrieReadSingleValue{RootHash: baseRoot, Path: path}
		retValue, err := forest.ReadSingleValue(read)
		require.NoError(t, err)
		if payload.IsEmpty() {
			require.Equal(t, 0, len(retValue))
		} else {
			require.Equal(t, payload.Value(), retValue)
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

	// Verify payloads are preserved
	read := &ledger.TrieRead{RootHash: baseRoot, Paths: paths}
	retValues, err := forest.Read(read) // reading from original Trie
	require.NoError(t, err)
	for i := range paths {
		require.Equal(t, retValues[i], payloads[i].Value())
	}

	readA := &ledger.TrieRead{RootHash: updatedRootA, Paths: pathsA}
	retValues, err = forest.Read(readA) // reading from updatedTrieA
	require.NoError(t, err)
	for i := range paths {
		require.Equal(t, retValues[i], payloadsA[i].Value())
	}

	readB := &ledger.TrieRead{RootHash: updatedRootB, Paths: pathsB}
	retValues, err = forest.Read(readB) // reading from updatedTrieB
	require.NoError(t, err)
	for i := range paths {
		require.Equal(t, retValues[i], payloadsB[i].Value())
	}
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
	retValuesA, err := forest.Read(read)
	require.NoError(t, err)
	for i := range paths {
		require.Equal(t, retValuesA[i], payloads[i].Value())
	}

	read = &ledger.TrieRead{RootHash: updatedRootB, Paths: paths}
	retValuesB, err := forest.Read(read)
	require.NoError(t, err)
	for i := range paths {
		require.Equal(t, retValuesB[i], payloads[i].Value())
	}
}

func TestNonExistingInvalidProof(t *testing.T) {
	forest, err := NewForest(5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)
	paths := testutils.RandomPaths(2)
	payloads := testutils.RandomPayloads(len(paths), 10, 20)

	existingPaths := paths[1:]
	existingPayloads := payloads[1:]

	nonExistingPaths := paths[:1]

	activeRoot := forest.GetEmptyRootHash()

	// adding a payload to a path
	update := &ledger.TrieUpdate{RootHash: activeRoot, Paths: existingPaths, Payloads: existingPayloads}
	activeRoot, err = forest.Update(update)
	require.NoError(t, err, "error updating")

	// reading proof for nonExistingPaths
	read := &ledger.TrieRead{RootHash: activeRoot, Paths: nonExistingPaths}
	batchProof, err := forest.Proofs(read)
	require.NoError(t, err, "error generating proofs")

	// now a malicious node modifies the proof to be Inclusion false
	// and change the proof such that one interim node has invalid hash
	batchProof.Proofs[0].Inclusion = false
	batchProof.Proofs[0].Interims[0] = hash.DummyHash

	// expect the VerifyTrieBatchProof should return false
	require.False(t, prf.VerifyTrieBatchProof(batchProof, ledger.State(activeRoot)))
}

// TestRandomUpdateReadProofValueSizes repeats a sequence of actions update, read, get value sizes, and proof random paths
// this simulates the common pattern of actions on flow
func TestRandomUpdateReadProofValueSizes(t *testing.T) {

	minPayloadByteSize := 2
	maxPayloadByteSize := 10
	rep := 10
	maxNumPathsPerStep := 10
	seed := time.Now().UnixNano()
	t.Log(seed)

	forest, err := NewForest(5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	activeRoot := forest.GetEmptyRootHash()
	require.NoError(t, err)
	latestPayloadByPath := make(map[ledger.Path]*ledger.Payload) // map store

	for e := 0; e < rep; e++ {
		paths := testutils.RandomPathsRandLen(maxNumPathsPerStep)
		payloads := testutils.RandomPayloads(len(paths), minPayloadByteSize, maxPayloadByteSize)

		// update map store with key values
		// we use this at the end of each step to check all existing keys
		for i, p := range paths {
			latestPayloadByPath[p] = payloads[i]
		}

		// test reading for non-existing keys
		nonExistingPaths := make([]ledger.Path, 0)
		otherPaths := testutils.RandomPathsRandLen(maxNumPathsPerStep)
		for _, p := range otherPaths {
			if _, ok := latestPayloadByPath[p]; !ok {
				nonExistingPaths = append(nonExistingPaths, p)
			}
		}
		read := &ledger.TrieRead{RootHash: activeRoot, Paths: nonExistingPaths}
		retValues, err := forest.Read(read)
		require.NoError(t, err, "error reading - non existing paths")
		for _, p := range retValues {
			require.Equal(t, 0, len(p))
		}

		// test value sizes for non-existent keys
		retValueSizes, err := forest.ValueSizes(read)
		require.NoError(t, err, "error value sizes - non existent paths")
		require.Equal(t, len(read.Paths), len(retValueSizes))
		for _, size := range retValueSizes {
			require.Equal(t, 0, size)
		}

		// test update
		update := &ledger.TrieUpdate{RootHash: activeRoot, Paths: paths, Payloads: payloads}
		activeRoot, err = forest.Update(update)
		require.NoError(t, err, "error updating")

		// test read
		read = &ledger.TrieRead{RootHash: activeRoot, Paths: paths}
		retValues, err = forest.Read(read)
		require.NoError(t, err, "error reading")
		for i := range payloads {
			require.Equal(t, retValues[i], payloads[i].Value())
		}

		// test value sizes for existing keys
		retValueSizes, err = forest.ValueSizes(read)
		require.NoError(t, err, "error value sizes")
		require.Equal(t, len(read.Paths), len(retValueSizes))
		for i := range payloads {
			require.Equal(t, payloads[i].Value().Size(), retValueSizes[i])
		}

		// test proof (mix of existing and non existing keys)
		proofPaths := make([]ledger.Path, 0)
		proofPaths = append(proofPaths, paths...)
		proofPaths = append(proofPaths, nonExistingPaths...)

		// shuffle the order of `proofPaths` to run `Proofs` on non-sorted input paths
		rand.Shuffle(len(proofPaths), func(i, j int) {
			proofPaths[i], proofPaths[j] = proofPaths[j], proofPaths[i]
		})

		// sort `proofPaths` into another slice
		sortedPaths := sortedCopy(proofPaths)

		read = &ledger.TrieRead{RootHash: activeRoot, Paths: proofPaths}
		batchProof, err := forest.Proofs(read)
		require.NoError(t, err, "error generating proofs")
		assert.True(t, prf.VerifyTrieBatchProof(batchProof, ledger.State(activeRoot)))

		// check `Proofs` has sorted the input paths.
		// this check is needed to not weaken the SPoCK secret entropy,
		// when `Proofs` is used to generate chunk data.
		assert.Equal(t, sortedPaths, read.Paths)

		// build a partial trie from batch proofs and check the root hash is equal
		psmt, err := ptrie.NewPSMT(activeRoot, batchProof)
		require.NoError(t, err, "error building partial trie")
		assert.Equal(t, psmt.RootHash(), activeRoot)

		// check payloads for all existing paths
		allPaths := make([]ledger.Path, 0, len(latestPayloadByPath))
		allPayloads := make([]*ledger.Payload, 0, len(latestPayloadByPath))
		for p, v := range latestPayloadByPath {
			allPaths = append(allPaths, p)
			allPayloads = append(allPayloads, v)
		}

		read = &ledger.TrieRead{RootHash: activeRoot, Paths: allPaths}
		retValues, err = forest.Read(read)
		require.NoError(t, err)
		for i, v := range allPayloads {
			assert.Equal(t, retValues[i], v.Value())
		}

		// check value sizes for all existing paths
		retValueSizes, err = forest.ValueSizes(read)
		require.NoError(t, err)
		require.Equal(t, len(read.Paths), len(retValueSizes))
		for i, v := range allPayloads {
			assert.Equal(t, v.Value().Size(), retValueSizes[i])
		}
	}
}

func sortedCopy(paths []ledger.Path) []ledger.Path {
	sortedPaths := make([]ledger.Path, len(paths))
	copy(sortedPaths, paths)
	sort.Slice(sortedPaths, func(i, j int) bool {
		return bytes.Compare(sortedPaths[i][:], sortedPaths[j][:]) < 0
	})
	return sortedPaths
}

// TestProofGenerationInclusion tests that inclusion proofs generated by a Trie pass verification
func TestProofGenerationInclusion(t *testing.T) {

	metricsCollector := &metrics.NoopCollector{}
	forest, err := NewForest(5, metricsCollector, nil)
	require.NoError(t, err)
	emptyRoot := forest.GetEmptyRootHash()

	p1 := pathByUint8s([]uint8{uint8(1), uint8(74)})
	v1 := payloadBySlices([]byte{'A'}, []byte{'A'})
	p2 := pathByUint8s([]uint8{uint8(2), uint8(74)})
	v2 := payloadBySlices([]byte{'B'}, []byte{'B'})
	p3 := pathByUint8s([]uint8{uint8(130), uint8(74)})
	v3 := payloadBySlices([]byte{'C'}, []byte{'C'})
	p4 := pathByUint8s([]uint8{uint8(131), uint8(74)})
	v4 := payloadBySlices([]byte{'D'}, []byte{'D'})
	paths := []ledger.Path{p1, p2, p3, p4}
	payloads := []*ledger.Payload{v1, v2, v3, v4}

	// shuffle the order of `proofPaths` to run `Proofs` on non-sorted input paths
	rand.Shuffle(len(paths), func(i, j int) {
		paths[i], paths[j] = paths[j], paths[i]
	})

	// sort `proofPaths` into another slice
	sortedPaths := sortedCopy(paths)

	update := &ledger.TrieUpdate{RootHash: emptyRoot, Paths: paths, Payloads: payloads}
	updatedRoot, err := forest.Update(update)
	require.NoError(t, err)
	read := &ledger.TrieRead{RootHash: updatedRoot, Paths: paths}
	proof, err := forest.Proofs(read)
	require.NoError(t, err)

	// verify batch proofs.
	assert.True(t, prf.VerifyTrieBatchProof(proof, ledger.State(updatedRoot)))

	// check `Proofs` has sorted the input paths.
	// this check is needed to not weaken the SPoCK secret entropy,
	// when `Proofs` is used to generate chunk data.
	assert.Equal(t, sortedPaths, read.Paths)
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

// TestValueSizesOrder tests returned value sizes are in the order as specified by the paths
func TestValueSizesOrder(t *testing.T) {

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

	// Get value sizes for paths {p1, p2}
	read := &ledger.TrieRead{RootHash: baseRoot, Paths: []ledger.Path{p1, p2}}
	retValueSizes, err := forest.ValueSizes(read)
	require.NoError(t, err)
	require.Equal(t, len(read.Paths), len(retValueSizes))
	require.Equal(t, v1.Value().Size(), retValueSizes[0])
	require.Equal(t, v2.Value().Size(), retValueSizes[1])

	// Get value sizes for paths {p2, p1}
	read = &ledger.TrieRead{RootHash: baseRoot, Paths: []ledger.Path{p2, p1}}
	retValueSizes, err = forest.ValueSizes(read)
	require.NoError(t, err)
	require.Equal(t, len(read.Paths), len(retValueSizes))
	require.Equal(t, v2.Value().Size(), retValueSizes[0])
	require.Equal(t, v1.Value().Size(), retValueSizes[1])
}

// TestMixGetValueSizes tests value sizes of a mix of set and unset registers.
// We expect value size 0 to be returned for unset registers.
func TestMixGetValueSizes(t *testing.T) {
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
	expectedValueSizes := []int{v1.Value().Size(), v2.Value().Size(), 0, 0}

	read := &ledger.TrieRead{RootHash: baseRoot, Paths: readPaths}
	retValueSizes, err := forest.ValueSizes(read)
	require.NoError(t, err)
	require.Equal(t, len(read.Paths), len(retValueSizes))
	for i := range read.Paths {
		require.Equal(t, expectedValueSizes[i], retValueSizes[i])
	}
}

// TestValueSizesWithDuplicatedKeys gets value sizes for two keys,
// where both keys have the same value.
// We expect to receive same value sizes twice.
func TestValueSizesWithDuplicatedKeys(t *testing.T) {
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
	expectedValueSizes := []int{v1.Value().Size(), v2.Value().Size(), v1.Value().Size()}

	read := &ledger.TrieRead{RootHash: baseRoot, Paths: readPaths}
	retValueSizes, err := forest.ValueSizes(read)
	require.NoError(t, err)
	require.Equal(t, len(read.Paths), len(retValueSizes))
	for i := range read.Paths {
		require.Equal(t, expectedValueSizes[i], retValueSizes[i])
	}
}

func TestPurgeCacheExcept(t *testing.T) {
	forest, err := NewForest(5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	nt := trie.NewEmptyMTrie()
	p1 := pathByUint8s([]uint8{uint8(53), uint8(74)})
	v1 := payloadBySlices([]byte{'A'}, []byte{'A'})

	updatedTrie1, _, err := trie.NewTrieWithUpdatedRegisters(nt, []ledger.Path{p1}, []ledger.Payload{*v1}, true)
	require.NoError(t, err)

	err = forest.AddTrie(updatedTrie1)
	require.NoError(t, err)

	p2 := pathByUint8s([]uint8{uint8(12), uint8(34)})
	v2 := payloadBySlices([]byte{'B'}, []byte{'B'})

	updatedTrie2, _, err := trie.NewTrieWithUpdatedRegisters(nt, []ledger.Path{p2}, []ledger.Payload{*v2}, true)
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
