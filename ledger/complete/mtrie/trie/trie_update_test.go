package trie_test

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
)

func Test_PathCreation(t *testing.T) {
	paths := []ledger.Path{
		testutils.PathByUint8(1 << 7),   // 1000...
		testutils.PathByUint8(1 << 6),   // 1000...
		testutils.PathByUint16(1 << 15), // 1000...
		testutils.PathByUint16(1 << 14), // 1000...
	}

	// emptyTrie := trie.NewEmptyMTrie()
	for _, path := range paths {
		fmt.Println(path)
	}
	// payload := testutils.LightPayload(11, 12345)
	var p ledger.Path
	binary.BigEndian.PutUint32(p[28:32], 1)
	fmt.Println(p)
}

// a full trie with 4 payloads at bottom
func Test_FullTrie4_Update(t *testing.T) {
	// prepare paths for a trie with only 2 levels, which has 4 payloads in total
	paths := []ledger.Path{
		testutils.PathByUint8(0),           // 00000000...
		testutils.PathByUint8(1 << 6),      // 01000000...
		testutils.PathByUint8(1 << 7),      // 10000000...
		testutils.PathByUint8(1<<7 + 1<<6), // 11000000...
	}

	// prepare payloads, the value doesn't matter
	payloads := make([]ledger.Payload, 0, len(paths))
	for i := range paths {
		payloads = append(payloads, *payloadFromInt16(uint16(i)))
	}

	t0 := trie.NewEmptyMTrie()
	payloadStorage := unittest.CreateMockPayloadStore()

	// create a full trie by adding payloads at all 4 paths to an empty trie
	t1, depth, err := trie.NewTrieWithUpdatedRegisters(t0, paths, payloads, true, payloadStorage)
	require.NoError(t, err)
	require.Equal(t, uint16(2), depth)

	// create a updated trie that is updated at one path,
	// so prepare the updated path first
	paths2 := []ledger.Path{
		paths[2], // 10000000...
	}
	payloads2 := []ledger.Payload{
		*payloadFromInt16(uint16(100)),
	}
	// create the updated trie at the given path
	t2, _, err := trie.NewTrieWithUpdatedRegisters(t1, paths2, payloads2, true, payloadStorage)
	require.NoError(t, err)

	// empty trie should have nothing stored at the path
	from0, err := t0.ReadSinglePayload(paths2[0], payloadStorage)
	require.NoError(t, err)
	require.True(t, from0.IsEmpty())

	// the full trie should have original payload at the path
	from1, err := t1.ReadSinglePayload(paths2[0], payloadStorage)
	require.NoError(t, err)
	require.True(t, from1.Equals(&payloads[2]))

	// the updated trie should have updated payload at the path
	from2, err := t2.ReadSinglePayload(paths2[0], payloadStorage)
	require.NoError(t, err)
	require.True(t, from2.Equals(&payloads2[0]))

}

func payloadFromInt16(i uint16) *ledger.Payload {
	return testutils.LightPayload(i, i)
}

// adding payloads to the trie until full
func Test_AddingPayloadUntilFull(t *testing.T) {

	// prepare paths for a trie with only 2 levels, which has 4 payloads in total
	paths := []ledger.Path{
		testutils.PathByUint8(0),           // 00000000...
		testutils.PathByUint8(1 << 6),      // 01000000...
		testutils.PathByUint8(1 << 7),      // 10000000...
		testutils.PathByUint8(1<<7 + 1<<6), // 11000000...
	}

	// prepare payloads, the value doesn't matter
	payloads := make([]ledger.Payload, 0, len(paths))
	for i := range paths {
		payloads = append(payloads, *payloadFromInt16(uint16(i)))
	}

	t0 := trie.NewEmptyMTrie()
	payloadStorage := unittest.CreateMockPayloadStore()
	fmt.Println(t0)

	fmt.Println("=== adding 1 payload at 00000000")
	t1, depth, err := trie.NewTrieWithUpdatedRegisters(t0, paths[0:1], payloads[0:1], true, payloadStorage)
	require.NoError(t, err)
	require.Equal(t, uint16(0), depth)
	fmt.Println(t1)

	fmt.Println("=== adding 1 payload at 01000000")
	t2, depth, err := trie.NewTrieWithUpdatedRegisters(t1, paths[1:2], payloads[1:2], true, payloadStorage)
	require.NoError(t, err)
	require.Equal(t, uint16(2), depth)
	fmt.Println(t2)

	emptyPayloads := []ledger.Payload{
		*ledger.EmptyPayload(),
	}

	fmt.Println("=== removing 1 payload at 01000000")
	t3, depth, err := trie.NewTrieWithUpdatedRegisters(t2, paths[1:2], emptyPayloads, true, payloadStorage)
	require.NoError(t, err)
	require.Equal(t, uint16(2), depth)
	fmt.Println(t3)
	require.Equal(t, t1, t3)

	fmt.Println("=== removing 1 payload at 00000000")
	t4, depth, err := trie.NewTrieWithUpdatedRegisters(t3, paths[0:1], emptyPayloads, true, payloadStorage)
	require.NoError(t, err)
	require.Equal(t, uint16(0), depth)
	fmt.Println(t4)
	require.Equal(t, t0, t4)
}
