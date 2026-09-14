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
	storeV6Checkpoint(t, dir, 1)

	require.NoError(t, requireV6Checkpoint(dir))
}

// TestRequireV6Checkpoint_V7 verifies that a directory whose latest checkpoint is
// V7 (payloadless) is rejected, since this command requires full payloads.
func TestRequireV6Checkpoint_V7(t *testing.T) {
	dir := t.TempDir()
	storeV7Checkpoint(t, dir, 1)

	err := requireV6Checkpoint(dir)
	require.Error(t, err)
	require.Contains(t, err.Error(), "V7")
}

// TestRequireV6Checkpoint_V6AndV7SameNumber verifies that a payloadless triedir
// holding both checkpoint.N (V6) and checkpoint.N.v7 for the same number is
// accepted: the WAL replay loads the V6 checkpoint, so the stats are correct.
func TestRequireV6Checkpoint_V6AndV7SameNumber(t *testing.T) {
	dir := t.TempDir()
	storeV6Checkpoint(t, dir, 1)
	storeV7Checkpoint(t, dir, 1)

	require.NoError(t, requireV6Checkpoint(dir))
}

// TestRequireV6Checkpoint_V7NewerThanV6 verifies that a strictly newer V7
// checkpoint is rejected even when older V6 checkpoints exist, since the WAL replay
// would silently fall back to an older V6 checkpoint and report stale stats.
func TestRequireV6Checkpoint_V7NewerThanV6(t *testing.T) {
	dir := t.TempDir()
	storeV6Checkpoint(t, dir, 1)
	storeV7Checkpoint(t, dir, 2)

	err := requireV6Checkpoint(dir)
	require.Error(t, err)
	require.Contains(t, err.Error(), "V7")
}

// storeV6Checkpoint writes a single-trie V6 checkpoint numbered `number` into dir.
func storeV6Checkpoint(t *testing.T, dir string, number int) {
	p := testutils.PathByUint8(0)
	v := testutils.LightPayload8('A', 'a')
	tr, _, err := trie.NewTrieWithUpdatedRegisters(
		trie.NewEmptyMTrie(), []ledger.Path{p}, []ledger.Payload{*v}, true)
	require.NoError(t, err)

	require.NoError(t, wal.StoreCheckpointV6Concurrently(
		[]*trie.MTrie{tr}, dir, wal.NumberToFilename(number), zerolog.Nop()))
}

// storeV7Checkpoint writes a single-trie V7 (payloadless) checkpoint numbered
// `number` into dir.
func storeV7Checkpoint(t *testing.T, dir string, number int) {
	p := testutils.PathByUint8(0)
	v := testutils.LightPayload8('A', 'a')
	tr, _, err := payloadless.NewTrieWithUpdatedRegisters(
		payloadless.NewEmptyMTrie(), []ledger.Path{p}, [][]byte{v.Value()}, true)
	require.NoError(t, err)

	require.NoError(t, wal.StoreCheckpointV7Concurrently(
		[]*payloadless.MTrie{tr}, dir, wal.NumberToFilenameV7(number), zerolog.Nop()))
}
