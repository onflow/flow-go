package pebble

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// sstableBootstrapWorkerCount is the worker count used by the tests of the sstable
// bootstrap, chosen greater than one to exercise processing several buckets in parallel.
const sstableBootstrapWorkerCount = 3

// TestRegisterBootstrapSSTables_IndexCheckpointFile_Happy bootstraps a register store from
// a checkpoint whose registers are spread over several buckets and checks that all registers
// are readable and that they were ingested without any flush or compaction.
func TestRegisterBootstrapSSTables_IndexCheckpointFile_Happy(t *testing.T) {
	t.Parallel()
	log := zerolog.New(io.Discard)
	rootHeight := uint64(10000)
	unittest.RunWithTempDir(t, func(dir string) {
		checkpointTrie, registerIDs := registerBootstrapTestTrie(t, 8, 100)
		rootHash := checkpointTrie.RootHash()
		fileName := "spread-checkpoint"
		require.NoErrorf(t, wal.StoreCheckpointV6Concurrently([]*trie.MTrie{checkpointTrie}, dir, fileName, log),
			"fail to store checkpoint")
		checkpointFile := path.Join(dir, fileName)
		pb, dbDir := createPebbleForTest(t)

		bootstrap, err := NewRegisterBootstrapSSTables(pb, dbDir, checkpointFile, rootHeight, rootHash, log)
		require.NoError(t, err)
		// force multiple sstables per bucket to exercise splitting them by size
		bootstrap.sstableTargetFileSize = 1 << 10
		bootstrap.maxBucketFileSize = 1 << 10
		require.NoError(t, bootstrap.IndexCheckpointFile(context.Background(), sstableBootstrapWorkerCount))

		require.Equal(t, uint64(len(registerIDs)), bootstrap.registerCount)
		require.Greater(t, bootstrap.sstableCount, 1)

		// all registers must be readable through the register store
		reg, err := NewRegisters(pb, PruningDisabled)
		require.NoError(t, err)
		require.Equal(t, rootHeight, reg.LatestHeight())
		require.Equal(t, rootHeight, reg.FirstHeight())

		for _, registerID := range registerIDs {
			val, err := reg.Get(registerID, rootHeight)
			require.NoError(t, err)
			require.Equal(t, []byte{defaultRegisterValue}, val)
		}

		// the registers must have been ingested straight into the lowest level: that is what
		// avoids rewriting the register store through the memtables and compactions
		metrics := pb.Metrics()
		require.Zero(t, metrics.Flush.Count)
		require.Zero(t, metrics.Compact.Count)
		for level := 0; level < 6; level++ {
			require.Zero(t, metrics.Levels[level].NumFiles, "level %d is not empty", level)
		}
		require.EqualValues(t, bootstrap.sstableCount, metrics.Levels[6].NumFiles)

		// the temporary bucket files and sstables must be cleaned up
		requireNoBootstrapTempDir(t, dbDir)

		require.NoError(t, pb.Close())
		require.NoError(t, os.RemoveAll(dbDir))
	})
}

// TestRegisterBootstrapSSTables_IndexCheckpointFile_SingleOwner bootstraps a register store from
// a checkpoint whose registers almost all belong to one owner, so that the bucket file of that
// owner cannot be split by the owner's bytes and is sorted in memory instead.
func TestRegisterBootstrapSSTables_IndexCheckpointFile_SingleOwner(t *testing.T) {
	t.Parallel()
	log := zerolog.New(io.Discard)
	rootHeight := uint64(10000)
	unittest.RunWithTempDir(t, func(dir string) {
		checkpointTrie, registerIDs := registerBootstrapTestTrie(t, 1, 1000)
		rootHash := checkpointTrie.RootHash()
		fileName := "single-owner-checkpoint"
		require.NoErrorf(t, wal.StoreCheckpointV6Concurrently([]*trie.MTrie{checkpointTrie}, dir, fileName, log),
			"fail to store checkpoint")
		checkpointFile := path.Join(dir, fileName)
		pb, dbDir := createPebbleForTest(t)

		bootstrap, err := NewRegisterBootstrapSSTables(pb, dbDir, checkpointFile, rootHeight, rootHash, log)
		require.NoError(t, err)
		// force the bucket files to be split by size, which the bucket file of the single owner
		// cannot be by the bytes of its owner
		bootstrap.sstableTargetFileSize = 1 << 10
		bootstrap.maxBucketFileSize = 1 << 10
		require.NoError(t, bootstrap.IndexCheckpointFile(context.Background(), sstableBootstrapWorkerCount))

		require.Equal(t, uint64(len(registerIDs)), bootstrap.registerCount)
		require.Greater(t, bootstrap.sstableCount, 1)

		reg, err := NewRegisters(pb, PruningDisabled)
		require.NoError(t, err)
		require.Equal(t, rootHeight, reg.FirstHeight())

		for _, registerID := range registerIDs {
			val, err := reg.Get(registerID, rootHeight)
			require.NoError(t, err)
			require.Equal(t, []byte{defaultRegisterValue}, val)
		}

		requireNoBootstrapTempDir(t, dbDir)

		require.NoError(t, pb.Close())
		require.NoError(t, os.RemoveAll(dbDir))
	})
}

// TestRegisterBootstrapSSTables_IndexCheckpointFile_Empty bootstraps a register store from
// a checkpoint without any register.
func TestRegisterBootstrapSSTables_IndexCheckpointFile_Empty(t *testing.T) {
	t.Parallel()
	log := zerolog.New(io.Discard)
	rootHeight := uint64(10000)
	unittest.RunWithTempDir(t, func(dir string) {
		emptyTrie := trie.NewEmptyMTrie()
		fileName := "empty-checkpoint"
		require.NoErrorf(t, wal.StoreCheckpointV6Concurrently([]*trie.MTrie{emptyTrie}, dir, fileName, log),
			"fail to store checkpoint")
		checkpointFile := path.Join(dir, fileName)
		pb, dbDir := createPebbleForTest(t)

		bootstrap, err := NewRegisterBootstrapSSTables(pb, dbDir, checkpointFile, rootHeight, emptyTrie.RootHash(), log)
		require.NoError(t, err)
		require.NoError(t, bootstrap.IndexCheckpointFile(context.Background(), sstableBootstrapWorkerCount))

		require.Zero(t, bootstrap.registerCount)
		require.Zero(t, bootstrap.sstableCount)

		reg, err := NewRegisters(pb, PruningDisabled)
		require.NoError(t, err)
		require.Equal(t, rootHeight, reg.LatestHeight())
		require.Equal(t, rootHeight, reg.FirstHeight())

		requireNoBootstrapTempDir(t, dbDir)

		require.NoError(t, pb.Close())
		require.NoError(t, os.RemoveAll(dbDir))
	})
}

// TestRegisterBootstrapSSTables_NewBootstrap checks that an already bootstrapped register
// store is rejected.
func TestRegisterBootstrapSSTables_NewBootstrap(t *testing.T) {
	t.Parallel()
	log := zerolog.New(io.Discard)
	rootHeight := uint64(1)
	rootHash := ledger.RootHash(unittest.StateCommitmentFixture())
	pb, dbDir := createPebbleForTest(t)

	require.NoError(t, initHeights(pb, rootHeight))
	_, err := NewRegisterBootstrapSSTables(pb, dbDir, path.Join(dbDir, "checkpoint"), rootHeight, rootHash, log)
	require.ErrorIs(t, err, ErrAlreadyBootstrapped)

	require.NoError(t, pb.Close())
	require.NoError(t, os.RemoveAll(dbDir))
}

// TestRegisterBootstrapSSTables_IndexCheckpointFile_InvalidPayloadKey checks that a
// checkpoint with an unexpected key format fails the bootstrap and leaves the register store
// unbootstrapped.
func TestRegisterBootstrapSSTables_IndexCheckpointFile_InvalidPayloadKey(t *testing.T) {
	t.Parallel()
	log := zerolog.New(io.Discard)
	rootHeight := uint64(666)
	unittest.RunWithTempDir(t, func(dir string) {
		// Two payloads whose keys are not owner/key pairs. The checkpoint leaf node reader
		// only reads the nodes below the subtrie level (the nodes above it are stored in a
		// separate part file), so the paths are chosen to differ deep below the subtrie
		// level, like the register paths of a real execution state do.
		payloads := []ledger.Payload{*testutils.LightPayload8('A', 'a'), *testutils.LightPayload8('B', 'b')}
		paths := []ledger.Path{testutils.PathByUint8(0), testutils.PathByUint8(1)}
		invalidTrie, _, err := trie.NewTrieWithUpdatedRegisters(trie.NewEmptyMTrie(), paths, payloads, true)
		require.NoError(t, err)

		fileName := "invalid-checkpoint"
		require.NoErrorf(t, wal.StoreCheckpointV6Concurrently([]*trie.MTrie{invalidTrie}, dir, fileName, log),
			"fail to store checkpoint")
		checkpointFile := path.Join(dir, fileName)
		pb, dbDir := createPebbleForTest(t)

		bootstrap, err := NewRegisterBootstrapSSTables(pb, dbDir, checkpointFile, rootHeight, invalidTrie.RootHash(), log)
		require.NoError(t, err)
		err = bootstrap.IndexCheckpointFile(context.Background(), sstableBootstrapWorkerCount)
		require.ErrorContains(t, err, "unexpected ledger key format")

		// a failed bootstrap does not initialize the heights, so the register store has to
		// be re-created before retrying
		isBootstrapped, err := IsBootstrapped(pb)
		require.NoError(t, err)
		require.False(t, isBootstrapped)
		requireNoBootstrapTempDir(t, dbDir)

		require.NoError(t, pb.Close())
		require.NoError(t, os.RemoveAll(dbDir))
	})
}

// TestRegisterBootstrapSSTables_IndexCheckpointFile_MissingPartFile checks that a checkpoint
// with a missing part file fails the bootstrap.
func TestRegisterBootstrapSSTables_IndexCheckpointFile_MissingPartFile(t *testing.T) {
	t.Parallel()
	log := zerolog.New(io.Discard)
	rootHeight := uint64(666)
	unittest.RunWithTempDir(t, func(dir string) {
		checkpointTrie, _ := registerBootstrapTestTrie(t, 8, 100)
		fileName := "incomplete-checkpoint"
		require.NoErrorf(t, wal.StoreCheckpointV6Concurrently([]*trie.MTrie{checkpointTrie}, dir, fileName, log),
			"fail to store checkpoint")
		require.NoError(t, os.Remove(path.Join(dir, fmt.Sprintf("%v.%03d", fileName, 2))))
		checkpointFile := path.Join(dir, fileName)
		pb, dbDir := createPebbleForTest(t)

		bootstrap, err := NewRegisterBootstrapSSTables(pb, dbDir, checkpointFile, rootHeight, checkpointTrie.RootHash(), log)
		require.NoError(t, err)
		err = bootstrap.IndexCheckpointFile(context.Background(), sstableBootstrapWorkerCount)
		require.ErrorIs(t, err, os.ErrNotExist)
		requireNoBootstrapTempDir(t, dbDir)

		require.NoError(t, pb.Close())
		require.NoError(t, os.RemoveAll(dbDir))
	})
}

// registerBootstrapTestTrie returns a trie with registers of `owners` owners, each owning
// `registersPerOwner` registers, plus a few global registers with an empty owner. The owners
// are chosen such that their registers are spread over several register bootstrap buckets.
// It returns the trie and the register IDs of all its registers.
func registerBootstrapTestTrie(t *testing.T, owners int, registersPerOwner int) (*trie.MTrie, []flow.RegisterID) {
	registerKeys := make([]ledger.Key, 0, owners*registersPerOwner+1)
	for owner := range owners {
		ownerBytes := make([]byte, flow.AddressLength)
		ownerBytes[0] = byte(owner * 16)
		for register := range registersPerOwner {
			registerKeys = append(registerKeys, ledger.Key{KeyParts: []ledger.KeyPart{
				{Type: ledger.KeyPartOwner, Value: ownerBytes},
				{Type: ledger.KeyPartKey, Value: fmt.Appendf(nil, "storage/reg%d", register)},
			}})
		}
	}
	// registers with an empty owner (global registers)
	registerKeys = append(registerKeys, ledger.Key{KeyParts: []ledger.KeyPart{
		{Type: ledger.KeyPartOwner, Value: nil},
		{Type: ledger.KeyPartKey, Value: []byte("uuid")},
	}})

	payloads := make([]ledger.Payload, 0, len(registerKeys))
	registerIDs := make([]flow.RegisterID, 0, len(registerKeys))
	for _, key := range registerKeys {
		payloads = append(payloads, *ledger.NewPayload(key, ledger.Value{defaultRegisterValue}))

		registerID, err := convert.LedgerKeyToRegisterID(key)
		require.NoError(t, err)
		registerIDs = append(registerIDs, registerID)
	}

	paths, err := pathfinder.KeysToPaths(registerKeys, 1)
	require.NoError(t, err)

	checkpointTrie, depth, err := trie.NewTrieWithUpdatedRegisters(
		trie.NewEmptyMTrie(), paths, payloads, false)
	require.NoError(t, err)
	require.GreaterOrEqual(t, depth, uint16(1))
	require.Len(t, checkpointTrie.AllPayloads(), len(registerKeys))
	return checkpointTrie, registerIDs
}

// requireNoBootstrapTempDir requires that no temporary directory of a bootstrap is left in
// the given register store directory.
func requireNoBootstrapTempDir(t *testing.T, registerDir string) {
	entries, err := os.ReadDir(registerDir)
	require.NoError(t, err)
	for _, entry := range entries {
		require.NotContains(t, entry.Name(), registerBootstrapTempDirPrefix)
	}
}
