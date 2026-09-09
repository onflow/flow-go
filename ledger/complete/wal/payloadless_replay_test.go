package wal

import (
	"os"
	"path"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/mtrie"
	"github.com/onflow/flow-go/ledger/complete/payloadless"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestReplayOnPayloadlessForest_IgnoresV6RootCheckpoint is a regression test for
// the case where a payloadless node boots with both a V7 root checkpoint (the
// real seed) and a V6 root.checkpoint present in the trie dir. The forest must
// be seeded from the V7 checkpoint, and the V6 root.checkpoint must NOT be read.
//
// To prove the V6 file is never touched, a corrupt root.checkpoint is placed
// alongside the V7 checkpoint: the previous implementation routed payloadless
// segment replay through [DiskWAL.replay], which falls back to loading the V6
// root checkpoint when replaying from segment 0 — that fallback would fail on
// the corrupt file. With the fix, the V6 file is ignored and replay succeeds.
func TestReplayOnPayloadlessForest_IgnoresV6RootCheckpoint(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		logger := zerolog.Nop()

		// Build a V7 root checkpoint from a simple trie and write it as the
		// payloadless root checkpoint (root.checkpoint.v7).
		v6Tries := createSimpleTrie(t)
		rootHash := v6Tries[0].RootHash()
		v7Tries, err := FromV6Tries(v6Tries)
		require.NoError(t, err)
		require.NoError(t, StoreCheckpointV7Concurrently(v7Tries, dir, RootCheckpointFilenameV7(), logger))

		// Place a corrupt V6 root checkpoint next to the V7 one. If the
		// payloadless replay path attempts to load it, the load fails — which is
		// exactly the regression this test guards against.
		junkPath := path.Join(dir, bootstrap.FilenameWALRootCheckpoint)
		require.NoError(t, os.WriteFile(junkPath, []byte("not a valid v6 checkpoint"), 0644))

		w, err := NewDiskWAL(logger, nil, metrics.NewNoopCollector(), dir, 10, pathByteSize, segmentSize)
		require.NoError(t, err)
		defer func() { <-w.Done() }()

		forest, err := payloadless.NewForest(100, &metrics.NoopCollector{}, nil)
		require.NoError(t, err)

		err = w.ReplayOnPayloadlessForest(forest)
		require.NoError(t, err, "replay must seed from V7 and must not load the V6 root checkpoint")

		require.True(t, forest.HasTrie(rootHash), "forest must be seeded from the V7 root checkpoint")
	})
}

// TestReplayOnPayloadlessForest_ReplaysWALSegments verifies that after seeding
// the forest from the V7 root checkpoint, WAL segment records that are newer
// than the checkpoint are still replayed onto the payloadless forest. This
// guards against the segment-replay refactor accidentally skipping segments.
func TestReplayOnPayloadlessForest_ReplaysWALSegments(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		logger := zerolog.Nop()

		// Seed state: a full forest with an initial update, captured as the V7
		// root checkpoint.
		fullForest, err := mtrie.NewForest(100, &metrics.NoopCollector{}, nil)
		require.NoError(t, err)

		paths0, payloads0 := randNPathPayloads(10)
		seed := &ledger.TrieUpdate{
			RootHash: fullForest.GetEmptyRootHash(),
			Paths:    paths0,
			Payloads: toPayloadPtrs(payloads0),
		}
		root0, err := fullForest.Update(seed)
		require.NoError(t, err)

		v6Tries, err := fullForest.GetTries()
		require.NoError(t, err)
		v7Tries, err := FromV6Tries(v6Tries)
		require.NoError(t, err)
		require.NoError(t, StoreCheckpointV7Concurrently(v7Tries, dir, RootCheckpointFilenameV7(), logger))

		// A second update, built on root0, recorded into the WAL but NOT in the
		// checkpoint. Replay must apply it to reach root1.
		paths1, payloads1 := randNPathPayloads(10)
		update1 := &ledger.TrieUpdate{
			RootHash: root0,
			Paths:    paths1,
			Payloads: toPayloadPtrs(payloads1),
		}
		root1, err := fullForest.Update(update1)
		require.NoError(t, err)

		// Record update1 into the WAL, then close to flush the segment to disk.
		recordWAL, err := NewDiskWAL(logger, nil, metrics.NewNoopCollector(), dir, 10, pathByteSize, segmentSize)
		require.NoError(t, err)
		_, _, err = recordWAL.RecordUpdate(update1)
		require.NoError(t, err)
		<-recordWAL.Done()

		// Replay on a fresh WAL: seed from V7 (root0), then replay the WAL
		// segment carrying update1 to reach root1.
		w, err := NewDiskWAL(logger, nil, metrics.NewNoopCollector(), dir, 10, pathByteSize, segmentSize)
		require.NoError(t, err)
		defer func() { <-w.Done() }()

		forest, err := payloadless.NewForest(100, &metrics.NoopCollector{}, nil)
		require.NoError(t, err)

		require.NoError(t, w.ReplayOnPayloadlessForest(forest))

		require.True(t, forest.HasTrie(root0), "forest must contain the V7 checkpoint root")
		require.True(t, forest.HasTrie(root1), "forest must contain the root produced by replaying the WAL segment")
	})
}
