package payloadless_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/ledger/complete/payloadless"
)

// Test_AllocatedRegCount verifies that AllocatedRegCount tracks all four allocated<->unallocated
// register transitions of an update: unallocated->allocated (delta +1), allocated->allocated with a
// different value (delta 0), allocated->unallocated (delta -1), and unallocated->unallocated (delta 0).
//
// Superseded once mtrie's TestTrieAllocatedRegCountRegSize is ported to this package (it exercises the
// same four transitions at scale); this is an interim, focused guard.
func Test_AllocatedRegCount(t *testing.T) {
	// Base trie: a minimal non-empty trie holding two allocated registers. With two registers the
	// root is an interim node, so an update to an existing register descends to its leaf and reaches
	// base case (1.a.i) there — the representative shape, rather than the empty-trie edge case.
	pathA := testutils.PathByUint16LeftPadded(0)
	pathB := testutils.PathByUint16LeftPadded(1)
	pathC := testutils.PathByUint16LeftPadded(2) // not allocated in the base trie
	base, _, err := payloadless.NewTrieWithUpdatedRegisters(
		payloadless.NewEmptyMTrie(),
		[]ledger.Path{pathA, pathB}, [][]byte{{0x01}, {0x11}}, true)
	require.NoError(t, err)
	require.Equal(t, uint64(2), base.AllocatedRegCount())

	t.Run("allocated -> allocated (different value): delta 0", func(t *testing.T) {
		updated, _, err := payloadless.NewTrieWithUpdatedRegisters(
			base, []ledger.Path{pathA}, [][]byte{{0x02}}, true)
		require.NoError(t, err)
		require.Equal(t, uint64(2), updated.AllocatedRegCount()) // count unchanged
		require.NotEqual(t, base.RootHash(), updated.RootHash()) // the value change took effect
	})

	t.Run("allocated -> unallocated: delta -1", func(t *testing.T) {
		updated, _, err := payloadless.NewTrieWithUpdatedRegisters(
			base, []ledger.Path{pathA}, [][]byte{{}}, true)
		require.NoError(t, err)
		require.Equal(t, uint64(1), updated.AllocatedRegCount()) // one register freed
		require.NotEqual(t, base.RootHash(), updated.RootHash())
	})

	t.Run("unallocated -> allocated: delta +1", func(t *testing.T) {
		updated, _, err := payloadless.NewTrieWithUpdatedRegisters(
			base, []ledger.Path{pathC}, [][]byte{{0x22}}, true)
		require.NoError(t, err)
		require.Equal(t, uint64(3), updated.AllocatedRegCount()) // one register allocated
		require.NotEqual(t, base.RootHash(), updated.RootHash())
	})

	t.Run("unallocated -> unallocated: delta 0", func(t *testing.T) {
		updated, _, err := payloadless.NewTrieWithUpdatedRegisters(
			base, []ledger.Path{pathC}, [][]byte{{}}, true)
		require.NoError(t, err)
		require.Equal(t, uint64(2), updated.AllocatedRegCount()) // count unchanged
		require.Equal(t, base.RootHash(), updated.RootHash())    // writing empty to a fresh path is a no-op
	})
}
