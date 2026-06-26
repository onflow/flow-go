package checkpoint_collect_stats

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/ledger/complete/payloadless"
	"github.com/onflow/flow-go/ledger/complete/wal"
)

// TestRequireV6Checkpoint_EmptyDir verifies that a directory without any numbered
// checkpoint is accepted (the caller proceeds with WAL replay / root checkpoint).
func TestRequireV6Checkpoint_EmptyDir(t *testing.T) {
	require.NoError(t, requireV6Checkpoint(t.TempDir()))
}

// TestRequireV6Checkpoint_V6 verifies that a directory whose latest checkpoint is
// V6 is accepted.
func TestRequireV6Checkpoint_V6(t *testing.T) {
	dir := t.TempDir()

	p := testutils.PathByUint8(0)
	v := testutils.LightPayload8('A', 'a')
	tr, _, err := trie.NewTrieWithUpdatedRegisters(
		trie.NewEmptyMTrie(), []ledger.Path{p}, []ledger.Payload{*v}, true)
	require.NoError(t, err)

	require.NoError(t, wal.StoreCheckpointV6Concurrently(
		[]*trie.MTrie{tr}, dir, wal.NumberToFilename(1), zerolog.Nop()))

	require.NoError(t, requireV6Checkpoint(dir))
}

// TestRequireV6Checkpoint_V7 verifies that a directory whose latest checkpoint is
// V7 (payloadless) is rejected, since this command requires full payloads.
func TestRequireV6Checkpoint_V7(t *testing.T) {
	dir := t.TempDir()

	p := testutils.PathByUint8(0)
	v := testutils.LightPayload8('A', 'a')
	tr, _, err := payloadless.NewTrieWithUpdatedRegisters(
		payloadless.NewEmptyMTrie(), []ledger.Path{p}, [][]byte{v.Value()}, true)
	require.NoError(t, err)

	require.NoError(t, wal.StoreCheckpointV7Concurrently(
		[]*payloadless.MTrie{tr}, dir, wal.NumberToFilenameV7(1), zerolog.Nop()))

	err = requireV6Checkpoint(dir)
	require.Error(t, err)
	require.Contains(t, err.Error(), "V7")
}
