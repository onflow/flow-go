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

// TestReplayOnPayloadlessForestUntil verifies the early-stop replay: it stops as
// soon as the target trie is produced (or is already in the V7 checkpoint), and
// reports whether the target was found. Stopping early is what lets an older
// state commitment be extracted without being evicted from the LRU forest by a
// full replay to the WAL tip.
func TestReplayOnPayloadlessForestUntil(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		logger := zerolog.Nop()

		// Seed state (root0) captured as the V7 root checkpoint.
		fullForest, err := mtrie.NewForest(100, &metrics.NoopCollector{}, nil)
		require.NoError(t, err)

		paths0, payloads0 := randNPathPayloads(10)
		root0, err := fullForest.Update(&ledger.TrieUpdate{
			RootHash: fullForest.GetEmptyRootHash(),
			Paths:    paths0,
			Payloads: toPayloadPtrs(payloads0),
		})
		require.NoError(t, err)

		v6Tries, err := fullForest.GetTries()
		require.NoError(t, err)
		v7Tries, err := FromV6Tries(v6Tries)
		require.NoError(t, err)
		require.NoError(t, StoreCheckpointV7Concurrently(v7Tries, dir, RootCheckpointFilenameV7(), logger))

		// Three chained updates recorded into the WAL (but NOT the checkpoint):
		// root0 -> root1 -> root2 -> root3.
		recordWAL, err := NewDiskWAL(logger, nil, metrics.NewNoopCollector(), dir, 10, pathByteSize, segmentSize)
		require.NoError(t, err)

		parent := root0
		roots := make([]ledger.RootHash, 0, 3)
		for range 3 {
			pathsi, payloadsi := randNPathPayloads(10)
			update := &ledger.TrieUpdate{
				RootHash: parent,
				Paths:    pathsi,
				Payloads: toPayloadPtrs(payloadsi),
			}
			root, err := fullForest.Update(update)
			require.NoError(t, err)
			_, _, err = recordWAL.RecordUpdate(update)
			require.NoError(t, err)
			roots = append(roots, root)
			parent = root
		}
		<-recordWAL.Done()
		root1, root2, root3 := roots[0], roots[1], roots[2]

		// A fourth update built on root3 but never recorded: a valid root hash that
		// is present neither in the checkpoint nor in the WAL.
		pathsAbsent, payloadsAbsent := randNPathPayloads(10)
		rootAbsent, err := fullForest.Update(&ledger.TrieUpdate{
			RootHash: root3,
			Paths:    pathsAbsent,
			Payloads: toPayloadPtrs(payloadsAbsent),
		})
		require.NoError(t, err)

		// replayUntil runs a fresh DiskWAL + forest and returns the found flag plus
		// the populated forest, so each case is independent.
		replayUntil := func(t *testing.T, target ledger.RootHash) (bool, *payloadless.Forest) {
			w, err := NewDiskWAL(logger, nil, metrics.NewNoopCollector(), dir, 10, pathByteSize, segmentSize)
			require.NoError(t, err)
			t.Cleanup(func() { <-w.Done() })

			forest, err := payloadless.NewForest(100, &metrics.NoopCollector{}, nil)
			require.NoError(t, err)

			found, err := w.ReplayOnPayloadlessForestUntil(forest, target)
			require.NoError(t, err)
			return found, forest
		}

		t.Run("stops at a mid-WAL target", func(t *testing.T) {
			found, forest := replayUntil(t, root1)
			require.True(t, found, "root1 is produced by replaying the first WAL update")
			require.True(t, forest.HasTrie(root1), "target trie must be present")
			// Proof of early-stop: updates producing root2/root3 must NOT be applied.
			require.False(t, forest.HasTrie(root2), "replay must stop at the target, before producing root2")
			require.False(t, forest.HasTrie(root3), "replay must stop at the target, before producing root3")
		})

		t.Run("target already in checkpoint replays no segments", func(t *testing.T) {
			found, forest := replayUntil(t, root0)
			require.True(t, found, "root0 is a checkpoint trie")
			require.True(t, forest.HasTrie(root0))
			require.False(t, forest.HasTrie(root1), "no WAL segment should be replayed when the target is in the checkpoint")
		})

		t.Run("target reachable only at the WAL tip", func(t *testing.T) {
			found, forest := replayUntil(t, root3)
			require.True(t, found, "root3 is produced by replaying all recorded WAL updates")
			require.True(t, forest.HasTrie(root3))
		})

		t.Run("absent target returns not found without error", func(t *testing.T) {
			found, forest := replayUntil(t, rootAbsent)
			require.False(t, found, "rootAbsent is present neither in the checkpoint nor the WAL")
			require.False(t, forest.HasTrie(rootAbsent))
		})
	})
}
