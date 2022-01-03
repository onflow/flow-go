package mtrie

import (
	"bytes"
	"fmt"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/encoding"
	prf "github.com/onflow/flow-go/ledger/common/proof"
	"github.com/onflow/flow-go/ledger/common/utils"
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

	updatedTrie, err := trie.NewTrieWithUpdatedRegisters(nt, []ledger.Path{p1}, []ledger.Payload{*v1}, true)
	require.NoError(t, err)

	// Add trie
	err = forest.AddTrie(updatedTrie)
	require.NoError(t, err)

	// Get trie
	retnt, err := forest.GetTrie(updatedTrie.RootHash())
	require.NoError(t, err)
	require.Equal(t, retnt.RootHash(), updatedTrie.RootHash())
	require.Equal(t, forest.Size(), 2)

	// Remove trie
	forest.RemoveTrie(updatedTrie.RootHash())
	require.Equal(t, forest.Size(), 1)
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
	require.Equal(t, baseTrie.MaxDepth(), uint16(2))
	require.Equal(t, baseTrie.AllocatedRegCount(), uint64(2))
	fmt.Println("BASE TRIE:")
	fmt.Println(baseTrie.String())

	p3 := pathByUint8s([]uint8{uint8(1), uint8(1)})
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
	require.Equal(t, baseTrie.MaxDepth(), uint16(2))
	require.Equal(t, baseTrie.AllocatedRegCount(), uint64(2))
	fmt.Println("BASE TRIE:")
	fmt.Println(baseTrie.String())

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
	require.Equal(t, updatedTrie.MaxDepth(), uint16(2))
	require.Equal(t, updatedTrie.AllocatedRegCount(), uint64(3))
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
	require.Equal(t, baseTrie.MaxDepth(), uint16(0))
	require.Equal(t, baseTrie.AllocatedRegCount(), uint64(1))
	fmt.Println("BASE TRIE:")
	fmt.Println(baseTrie.String())

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
	require.Equal(t, updatedTrie.MaxDepth(), uint16(7))
	require.Equal(t, updatedTrie.AllocatedRegCount(), uint64(2))
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
	require.Equal(t, baseTrie.MaxDepth(), uint16(2))
	require.Equal(t, baseTrie.AllocatedRegCount(), uint64(3))
	fmt.Println("BASE TRIE:")
	fmt.Println(baseTrie.String())

	// we update value for path p1 and in addition add p3 that has the same prefix `10` as p0
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
	require.Equal(t, updatedTrie.MaxDepth(), uint16(3))
	require.Equal(t, updatedTrie.AllocatedRegCount(), uint64(4))
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
	forest, err := NewForest(5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	// path: 000...0000000100000000
	p1 := utils.PathByUint16LeftPadded(256)
	v1 := payloadBySlices([]byte{'A'}, []byte{'A'})

	// path: 000...0000000100000001
	p2 := utils.PathByUint16LeftPadded(257)
	v2 := payloadBySlices([]byte{'B'}, []byte{'B'})

	paths := []ledger.Path{p1, p2}
	payloads := []*ledger.Payload{v1, v2}
	update := &ledger.TrieUpdate{RootHash: forest.GetEmptyRootHash(), Paths: paths, Payloads: payloads}
	updatedRoot, err := forest.Update(update)
	require.NoError(t, err)

	updatedTrie, err := forest.GetTrie(updatedRoot)
	require.NoError(t, err)
	require.Equal(t, updatedTrie.MaxDepth(), uint16(256))
	require.Equal(t, updatedTrie.AllocatedRegCount(), uint64(2))
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
	retPayloads, err := forest.Read(read)
	require.NoError(t, err)
	require.True(t, bytes.Equal(encoding.EncodePayload(retPayloads[0]), encoding.EncodePayload(payloads[0])))

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
	retPayloads, err := forest.Read(read)
	require.NoError(t, err)
	require.True(t, bytes.Equal(encoding.EncodePayload(retPayloads[0]), encoding.EncodePayload(v2)))

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
	require.Equal(t, v0, data[0])

	// modify returned slice
	data[0].Value = []byte("new value")

	// read again
	data2, err := forest.Read(read)
	require.NoError(t, err)
	require.Len(t, data2, 1)
	require.Equal(t, v0, data2[0])
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
	retPayloads, err := forest.Read(read)
	require.NoError(t, err)
	require.Equal(t, len(retPayloads), len(payloads))
	require.True(t, bytes.Equal(encoding.EncodePayload(retPayloads[0]), encoding.EncodePayload(payloads[0])))
	require.True(t, bytes.Equal(encoding.EncodePayload(retPayloads[1]), encoding.EncodePayload(payloads[1])))

	read = &ledger.TrieRead{RootHash: testRoot, Paths: []ledger.Path{p2, p1}}
	retPayloads, err = forest.Read(read)
	require.NoError(t, err)
	require.Equal(t, len(retPayloads), len(payloads))
	require.True(t, bytes.Equal(encoding.EncodePayload(retPayloads[1]), encoding.EncodePayload(payloads[0])))
	require.True(t, bytes.Equal(encoding.EncodePayload(retPayloads[0]), encoding.EncodePayload(payloads[1])))
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
	retPayloads, err := forest.Read(read)
	require.NoError(t, err)
	for i := range paths {
		require.True(t, bytes.Equal(encoding.EncodePayload(retPayloads[i]), encoding.EncodePayload(expectedPayloads[i])))
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
	retPayloads, err := forest.Read(read)
	require.NoError(t, err)
	require.Equal(t, len(expectedPayloads), len(retPayloads))
	for i := range paths {
		require.True(t, bytes.Equal(encoding.EncodePayload(retPayloads[i]), encoding.EncodePayload(expectedPayloads[i])))
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
	retPayloads, err := forest.Read(read)
	require.NoError(t, err)
	require.True(t, retPayloads[0].IsEmpty())
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
// this simulates the common pattern of actions on flow
func TestRandomUpdateReadProof(t *testing.T) {

	minPayloadByteSize := 2
	maxPayloadByteSize := 10
	rep := 10
	maxNumPathsPerStep := 10
	seed := time.Now().UnixNano()
	rand.Seed(seed)
	t.Log(seed)

	forest, err := NewForest(5, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	activeRoot := forest.GetEmptyRootHash()
	require.NoError(t, err)
	latestPayloadByPath := make(map[ledger.Path]*ledger.Payload) // map store

	for e := 0; e < rep; e++ {
		paths := utils.RandomPathsRandLen(maxNumPathsPerStep)
		payloads := utils.RandomPayloads(len(paths), minPayloadByteSize, maxPayloadByteSize)

		// update map store with key values
		// we use this at the end of each step to check all existing keys
		for i, p := range paths {
			latestPayloadByPath[p] = payloads[i]
		}

		// test reading for non-existing keys
		nonExistingPaths := make([]ledger.Path, 0)
		otherPaths := utils.RandomPathsRandLen(maxNumPathsPerStep)
		for _, p := range otherPaths {
			if _, ok := latestPayloadByPath[p]; !ok {
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
		retPayloads, err = forest.Read(read)
		require.NoError(t, err)
		for i, v := range allPayloads {
			assert.True(t, v.Equals(retPayloads[i]))
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
	return &ledger.Payload{Key: key, Value: value}
}

func pathByUint8s(inputs []uint8) ledger.Path {
	var b ledger.Path
	copy(b[:], inputs)
	return b
}
