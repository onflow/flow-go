package checkpoint_list_tries

import (
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/ledger/complete/payloadless"
	"github.com/onflow/flow-go/ledger/complete/wal"
)

// TestReadTrieRootHashesV6 verifies that the trie root hashes are read from a V6
// checkpoint in the order they were stored, without loading the full forest.
func TestReadTrieRootHashesV6(t *testing.T) {
	dir := t.TempDir()
	const fileName = "checkpoint"

	tries := createV6Tries(t)

	err := wal.StoreCheckpointV6Concurrently(tries, dir, fileName, zerolog.Nop())
	require.NoError(t, err)

	hashes, err := readTrieRootHashes(zerolog.Nop(), filepath.Join(dir, fileName))
	require.NoError(t, err)

	expected := make([]ledger.RootHash, len(tries))
	for i, tr := range tries {
		expected[i] = tr.RootHash()
	}
	require.Equal(t, expected, hashes)
}

// TestReadTrieRootHashesV7 verifies that the trie root hashes are read from a V7
// (payloadless) checkpoint in the order they were stored, by dispatching on the
// V7 filename suffix.
func TestReadTrieRootHashesV7(t *testing.T) {
	dir := t.TempDir()
	fileName := "checkpoint" + wal.V7FileSuffix

	tries := createV7Tries(t)

	err := wal.StoreCheckpointV7Concurrently(tries, dir, fileName, zerolog.Nop())
	require.NoError(t, err)

	hashes, err := readTrieRootHashes(zerolog.Nop(), filepath.Join(dir, fileName))
	require.NoError(t, err)

	expected := make([]ledger.RootHash, len(tries))
	for i, tr := range tries {
		expected[i] = tr.RootHash()
	}
	require.Equal(t, expected, hashes)
}

// createV6Tries builds a chain of two distinct full-payload tries for use as V6
// checkpoint content.
func createV6Tries(t *testing.T) []*trie.MTrie {
	p1 := testutils.PathByUint8(0)
	v1 := testutils.LightPayload8('A', 'a')
	trie1, _, err := trie.NewTrieWithUpdatedRegisters(
		trie.NewEmptyMTrie(), []ledger.Path{p1}, []ledger.Payload{*v1}, true)
	require.NoError(t, err)

	p2 := testutils.PathByUint8(1)
	v2 := testutils.LightPayload8('B', 'b')
	trie2, _, err := trie.NewTrieWithUpdatedRegisters(
		trie1, []ledger.Path{p2}, []ledger.Payload{*v2}, true)
	require.NoError(t, err)

	return []*trie.MTrie{trie1, trie2}
}

// createV7Tries builds a chain of two distinct payloadless tries for use as V7
// checkpoint content.
func createV7Tries(t *testing.T) []*payloadless.MTrie {
	p1 := testutils.PathByUint8(0)
	v1 := testutils.LightPayload8('A', 'a')
	trie1, _, err := payloadless.NewTrieWithUpdatedRegisters(
		payloadless.NewEmptyMTrie(), []ledger.Path{p1}, [][]byte{v1.Value()}, true)
	require.NoError(t, err)

	p2 := testutils.PathByUint8(1)
	v2 := testutils.LightPayload8('B', 'b')
	trie2, _, err := payloadless.NewTrieWithUpdatedRegisters(
		trie1, []ledger.Path{p2}, [][]byte{v2.Value()}, true)
	require.NoError(t, err)

	return []*payloadless.MTrie{trie1, trie2}
}
