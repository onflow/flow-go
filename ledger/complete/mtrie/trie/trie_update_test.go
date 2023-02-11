package trie_test

import (
	"fmt"
	"testing"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/stretchr/testify/require"
)

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
	fmt.Println(t0)

	fmt.Println("=== adding 1 payload at 00000000")
	t1, depth, err := trie.NewTrieWithUpdatedRegisters(t0, paths[0:1], payloads[0:1], true)
	require.NoError(t, err)
	require.Equal(t, uint16(0), depth)
	fmt.Println(t1)

	fmt.Println("=== adding 1 payload at 01000000")
	t2, depth, err := trie.NewTrieWithUpdatedRegisters(t1, paths[1:2], payloads[1:2], true)
	require.NoError(t, err)
	require.Equal(t, uint16(2), depth)
	fmt.Println(t2)

	emptyPayloads := []ledger.Payload{
		*ledger.EmptyPayload(),
	}

	fmt.Println("=== removing 1 payload at 01000000")
	t3, depth, err := trie.NewTrieWithUpdatedRegisters(t2, paths[1:2], emptyPayloads, true)
	require.NoError(t, err)
	require.Equal(t, uint16(2), depth)
	fmt.Println(t3)
	require.Equal(t, t1, t3)

	fmt.Println("=== removing 1 payload at 00000000")
	t4, depth, err := trie.NewTrieWithUpdatedRegisters(t3, paths[0:1], emptyPayloads, true)
	require.NoError(t, err)
	require.Equal(t, uint16(0), depth)
	fmt.Println(t4)

	// expect the trie is empty
	require.Equal(t, t0, t4)
}

func payloadFromInt16(i uint16) *ledger.Payload {
	return testutils.LightPayload(i, i)
}
