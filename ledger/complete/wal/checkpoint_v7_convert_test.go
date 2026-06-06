package wal

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/ledger/complete/mtrie"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/ledger/complete/payloadless"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestFromV6LeafNode_PreservesHash converts a V6 leaf node into a V7 leaf and
// verifies the node hash is preserved.
func TestFromV6LeafNode_PreservesHash(t *testing.T) {
	// Build a single-register V6 trie and grab its (compactified) leaf root.
	emptyTrie := trie.NewEmptyMTrie()
	p := testutils.PathByUint8(0)
	v := testutils.LightPayload8('A', 'a')

	updatedTrie, _, err := trie.NewTrieWithUpdatedRegisters(
		emptyTrie, []ledger.Path{p}, []ledger.Payload{*v}, true,
	)
	require.NoError(t, err)
	v6Root := updatedTrie.RootNode()
	require.True(t, v6Root.IsLeaf(), "expected compactified leaf root for single-register trie")

	converted, err := FromV6LeafNode(v6Root)
	require.NoError(t, err)
	require.Equal(t, v6Root.Hash(), converted.Hash(), "leaf node hash must be preserved across V6→V7 conversion")
	require.Equal(t, v6Root.Height(), converted.Height())
	require.Equal(t, *v6Root.Path(), *converted.Path())
	require.NotNil(t, converted.LeafHash(), "allocated leaf must have a non-nil leafHash")
}

// TestFromV6LeafNode_RejectsInterim verifies that calling FromV6LeafNode on an
// interim V6 node returns an error.
func TestFromV6LeafNode_RejectsInterim(t *testing.T) {
	emptyTrie := trie.NewEmptyMTrie()
	paths, payloads := randNPathPayloads(10)
	updated, _, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths, payloads, true)
	require.NoError(t, err)

	root := updated.RootNode()
	require.False(t, root.IsLeaf(), "test setup expects an interim root")

	_, err = FromV6LeafNode(root)
	require.Error(t, err, "FromV6LeafNode must reject interim nodes")
}

// TestFromV6Trie_PreservesRootHash builds a V6 trie with multiple registers and
// verifies that the converted V7 trie has the same root hash.
func TestFromV6Trie_PreservesRootHash(t *testing.T) {
	emptyTrie := trie.NewEmptyMTrie()
	paths, payloads := randNPathPayloads(50)
	v6Trie, _, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths, payloads, true)
	require.NoError(t, err)

	v7Trie, err := FromV6Trie(v6Trie)
	require.NoError(t, err)
	require.Equal(t, v6Trie.RootHash(), v7Trie.RootHash(), "V7 root hash must match V6 root hash")
	require.Equal(t, v6Trie.AllocatedRegCount(), v7Trie.AllocatedRegCount())
}

// TestFromV6Trie_Empty verifies that converting an empty V6 trie produces an
// empty V7 trie.
func TestFromV6Trie_Empty(t *testing.T) {
	v6Empty := trie.NewEmptyMTrie()
	v7, err := FromV6Trie(v6Empty)
	require.NoError(t, err)
	require.True(t, v7.IsEmpty())
	require.Equal(t, v6Empty.RootHash(), v7.RootHash())
}

// TestFromV6Tries_SharedSubtries verifies that converting a slice of V6 tries
// with shared sub-tries preserves every root hash and exercises the
// memoization path.
func TestFromV6Tries_SharedSubtries(t *testing.T) {
	tries := make([]*trie.MTrie, 0)
	active := trie.NewEmptyMTrie()
	for i := 0; i < 5; i++ {
		paths, payloads := randNPathPayloads(30)
		var err error
		active, _, err = trie.NewTrieWithUpdatedRegisters(active, paths, payloads, false)
		require.NoError(t, err)
		tries = append(tries, active)
	}

	converted, err := FromV6Tries(tries)
	require.NoError(t, err)
	require.Equal(t, len(tries), len(converted))
	for i, v6 := range tries {
		require.Equal(t, v6.RootHash(), converted[i].RootHash(), "trie %d root hash mismatch", i)
	}
}

// TestConvertCheckpointV6ToV7_PreservesRootHashes writes a V6 checkpoint to disk,
// runs ConvertCheckpointV6ToV7, then reads the V7 result and verifies every
// trie root hash matches.
func TestConvertCheckpointV6ToV7_PreservesRootHashes(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		logger := zerolog.Nop()
		v6Tries := createMultipleRandomTries(t)

		v6Name := "checkpoint.00000100"
		require.NoError(t, StoreCheckpointV6Concurrently(v6Tries, dir, v6Name, logger))

		v7Name := v6Name + V7FileSuffix
		require.NoError(t, ConvertCheckpointV6ToV7(dir, v6Name, dir, v7Name, logger, 16))

		v7Tries, err := OpenAndReadCheckpointV7(dir, v7Name, logger)
		require.NoError(t, err)
		require.Equal(t, len(v6Tries), len(v7Tries))
		for i, v6 := range v6Tries {
			require.Equal(t, v6.RootHash(), v7Tries[i].RootHash(), "trie %d root hash mismatch", i)
		}
	})
}

// TestConvertCheckpointV6ToV7_NWorkerOne verifies the converter works with the
// minimum permitted nWorker value (=1).
func TestConvertCheckpointV6ToV7_NWorkerOne(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		logger := zerolog.Nop()
		v6Tries := createMultipleRandomTries(t)

		v6Name := "checkpoint.00000200"
		require.NoError(t, StoreCheckpointV6Concurrently(v6Tries, dir, v6Name, logger))

		v7Name := v6Name + V7FileSuffix
		require.NoError(t, ConvertCheckpointV6ToV7(dir, v6Name, dir, v7Name, logger, 1))

		v7Tries, err := OpenAndReadCheckpointV7(dir, v7Name, logger)
		require.NoError(t, err)
		for i, v6 := range v6Tries {
			require.Equal(t, v6.RootHash(), v7Tries[i].RootHash())
		}
	})
}

// TestConvertCheckpointV6ToV7_InvalidNWorker verifies argument validation.
func TestConvertCheckpointV6ToV7_InvalidNWorker(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		logger := zerolog.Nop()
		err := ConvertCheckpointV6ToV7(dir, "doesnt-matter", dir, "out"+V7FileSuffix, logger, 0)
		require.Error(t, err, "nWorker=0 must be rejected")

		err = ConvertCheckpointV6ToV7(dir, "doesnt-matter", dir, "out"+V7FileSuffix, logger, 17)
		require.Error(t, err, "nWorker > subtrieCount must be rejected")
	})
}

// TestConvertCheckpointV6ToV7_RequiresV7Suffix verifies that the converter
// refuses to write an output file without the V7 suffix.
func TestConvertCheckpointV6ToV7_RequiresV7Suffix(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		logger := zerolog.Nop()
		v6Tries := createSimpleTrie(t)
		v6Name := "checkpoint.00000001"
		require.NoError(t, StoreCheckpointV6Concurrently(v6Tries, dir, v6Name, logger))

		err := ConvertCheckpointV6ToV7(dir, v6Name, dir, "no-suffix", logger, 4)
		require.Error(t, err, "output filename without V7 suffix must be rejected")
	})
}

// TestConvertCheckpointV6ToV7_RejectsClobber verifies that the converter
// refuses to overwrite an existing V7 output.
func TestConvertCheckpointV6ToV7_RejectsClobber(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		logger := zerolog.Nop()
		v6Tries := createSimpleTrie(t)
		v6Name := "checkpoint.00000002"
		require.NoError(t, StoreCheckpointV6Concurrently(v6Tries, dir, v6Name, logger))

		v7Name := v6Name + V7FileSuffix
		require.NoError(t, ConvertCheckpointV6ToV7(dir, v6Name, dir, v7Name, logger, 4))

		err := ConvertCheckpointV6ToV7(dir, v6Name, dir, v7Name, logger, 4)
		require.Error(t, err, "second conversion to the same V7 output must be rejected")
	})
}

// TestConvertCheckpointV6ToV7_MissingV6Input verifies that the converter
// returns an error when the V6 source is missing.
func TestConvertCheckpointV6ToV7_MissingV6Input(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		logger := zerolog.Nop()
		err := ConvertCheckpointV6ToV7(dir, "missing", dir, "missing"+V7FileSuffix, logger, 4)
		require.Error(t, err, "missing V6 input must be reported")
	})
}

// TestConvertCheckpointV6ToV7_DifferentOutputDir verifies that the converter
// writes to a different output directory when one is supplied.
func TestConvertCheckpointV6ToV7_DifferentOutputDir(t *testing.T) {
	unittest.RunWithTempDir(t, func(srcDir string) {
		unittest.RunWithTempDir(t, func(dstDir string) {
			logger := zerolog.Nop()
			v6Tries := createSimpleTrie(t)
			v6Name := "checkpoint.00000003"
			require.NoError(t, StoreCheckpointV6Concurrently(v6Tries, srcDir, v6Name, logger))

			v7Name := v6Name + V7FileSuffix
			require.NoError(t, ConvertCheckpointV6ToV7(srcDir, v6Name, dstDir, v7Name, logger, 4))

			// V7 files exist in dstDir, not in srcDir.
			v7Tries, err := OpenAndReadCheckpointV7(dstDir, v7Name, logger)
			require.NoError(t, err)
			for i, v6 := range v6Tries {
				require.Equal(t, v6.RootHash(), v7Tries[i].RootHash())
			}
			// The original V6 still loads from the source dir.
			loaded, err := LoadCheckpoint(filepath.Join(srcDir, v6Name), logger)
			require.NoError(t, err)
			require.Equal(t, len(v6Tries), len(loaded))
		})
	})
}

// TestConvertCheckpointV6ToV7_EmptyTrie verifies the converter handles an
// empty-trie checkpoint.
func TestConvertCheckpointV6ToV7_EmptyTrie(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		logger := zerolog.Nop()
		v6Tries := []*trie.MTrie{trie.NewEmptyMTrie()}
		v6Name := "checkpoint.00000004"
		require.NoError(t, StoreCheckpointV6Concurrently(v6Tries, dir, v6Name, logger))

		v7Name := v6Name + V7FileSuffix
		require.NoError(t, ConvertCheckpointV6ToV7(dir, v6Name, dir, v7Name, logger, 16))

		v7Tries, err := OpenAndReadCheckpointV7(dir, v7Name, logger)
		require.NoError(t, err)
		require.Len(t, v7Tries, 1)
		require.True(t, v7Tries[0].IsEmpty())
	})
}

// TestFullVsPayloadlessForest_SingleUpdate verifies that applying the same
// TrieUpdate to an empty full forest and an empty payloadless forest produces
// the same root hash.
func TestFullVsPayloadlessForest_SingleUpdate(t *testing.T) {
	const forestCapacity = 100
	fullForest, err := mtrie.NewForest(forestCapacity, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)
	plForest, err := payloadless.NewForest(forestCapacity, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	paths, payloads := randNPathPayloads(50)
	update := &ledger.TrieUpdate{
		RootHash: fullForest.GetEmptyRootHash(),
		Paths:    paths,
		Payloads: toPayloadPtrs(payloads),
	}
	fullRoot, err := fullForest.Update(update)
	require.NoError(t, err)

	// Payloadless forest uses the same TrieUpdate API.
	plUpdate := &ledger.TrieUpdate{
		RootHash: plForest.GetEmptyRootHash(),
		Paths:    paths,
		Payloads: toPayloadPtrs(payloads),
	}
	plRoot, err := plForest.Update(plUpdate)
	require.NoError(t, err)

	require.Equal(t, fullRoot, plRoot, "single update root hash must match across full and payloadless forests")
}

// TestFullVsPayloadlessForest_IncrementalUpdates applies several rounds of
// updates (mix of inserts, updates, and deletions) to both a full and a
// payloadless forest in lockstep and verifies the root hashes stay in sync.
func TestFullVsPayloadlessForest_IncrementalUpdates(t *testing.T) {
	const forestCapacity = 100
	fullForest, err := mtrie.NewForest(forestCapacity, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)
	plForest, err := payloadless.NewForest(forestCapacity, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	fullRoot := fullForest.GetEmptyRootHash()
	plRoot := plForest.GetEmptyRootHash()
	require.Equal(t, fullRoot, plRoot, "empty root hashes must match")

	// Track allocated paths so we can also apply deletions (empty payloads).
	allocated := make([]ledger.Path, 0)

	for round := 0; round < 8; round++ {
		// New writes for this round.
		paths, payloads := randNPathPayloads(20)
		allocated = append(allocated, paths...)

		// Mix in some "deletions" (empty-value writes) for previously-allocated paths.
		if round > 0 && len(allocated) >= 5 {
			for i := 0; i < 5; i++ {
				paths = append(paths, allocated[i])
				payloads = append(payloads, *ledger.EmptyPayload())
			}
		}

		fullUpdate := &ledger.TrieUpdate{
			RootHash: fullRoot,
			Paths:    paths,
			Payloads: toPayloadPtrs(payloads),
		}
		plUpdate := &ledger.TrieUpdate{
			RootHash: plRoot,
			Paths:    paths,
			Payloads: toPayloadPtrs(payloads),
		}

		fullRoot, err = fullForest.Update(fullUpdate)
		require.NoError(t, err, "full forest update failed at round %d", round)
		plRoot, err = plForest.Update(plUpdate)
		require.NoError(t, err, "payloadless forest update failed at round %d", round)

		require.Equal(t, fullRoot, plRoot, "root hash diverged at round %d", round)
	}
}

// TestFullVsPayloadlessForest_LoadConvertedCheckpoint takes a V6 forest state,
// writes it out, converts to V7, loads the V7 into a payloadless forest, and
// applies further updates to both forests in parallel — verifying they stay
// in sync after a real checkpoint round-trip.
func TestFullVsPayloadlessForest_LoadConvertedCheckpoint(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		logger := zerolog.Nop()
		const forestCapacity = 100

		fullForest, err := mtrie.NewForest(forestCapacity, &metrics.NoopCollector{}, nil)
		require.NoError(t, err)
		plForest, err := payloadless.NewForest(forestCapacity, &metrics.NoopCollector{}, nil)
		require.NoError(t, err)

		// Seed both forests with the same initial state.
		paths, payloads := randNPathPayloads(40)
		seed := &ledger.TrieUpdate{
			RootHash: fullForest.GetEmptyRootHash(),
			Paths:    paths,
			Payloads: toPayloadPtrs(payloads),
		}
		fullRoot, err := fullForest.Update(seed)
		require.NoError(t, err)
		plRoot, err := plForest.Update(seed)
		require.NoError(t, err)
		require.Equal(t, fullRoot, plRoot)

		// Snapshot the full forest as a V6 checkpoint.
		v6Tries, err := fullForest.GetTries()
		require.NoError(t, err)
		v6Name := "checkpoint.00000005"
		require.NoError(t, StoreCheckpointV6Concurrently(v6Tries, dir, v6Name, logger))

		// Convert V6 → V7.
		v7Name := v6Name + V7FileSuffix
		require.NoError(t, ConvertCheckpointV6ToV7(dir, v6Name, dir, v7Name, logger, 16))

		// Reload V7 into a fresh payloadless forest.
		v7Tries, err := OpenAndReadCheckpointV7(dir, v7Name, logger)
		require.NoError(t, err)
		freshPlForest, err := payloadless.NewForest(forestCapacity, &metrics.NoopCollector{}, nil)
		require.NoError(t, err)
		require.NoError(t, freshPlForest.AddTries(v7Tries))

		// Verify the loaded V7 forest contains a trie matching the seed root.
		require.True(t, freshPlForest.HasTrie(fullRoot), "fresh payloadless forest must contain the seed root hash")

		// Apply identical follow-up updates to both forests starting from the
		// seed root that both share.
		fullRoot, err = fullForest.MostRecentTouchedRootHash()
		require.NoError(t, err)

		for round := 0; round < 4; round++ {
			updatePaths, updatePayloads := randNPathPayloads(15)
			update := &ledger.TrieUpdate{
				RootHash: fullRoot,
				Paths:    updatePaths,
				Payloads: toPayloadPtrs(updatePayloads),
			}
			fullRoot, err = fullForest.Update(update)
			require.NoError(t, err)

			update.RootHash = plRoot
			plRoot, err = freshPlForest.Update(update)
			require.NoError(t, err)

			require.Equal(t, fullRoot, plRoot, "root hash diverged after checkpoint round-trip at round %d", round)
		}
	})
}

// TestFullVsPayloadlessForest_DeterministicRandom replays the same random
// updates against both forests with a deterministic seed (via crypto/rand for
// values, fixed paths) and checks every intermediate root hash.
func TestFullVsPayloadlessForest_DeterministicRandom(t *testing.T) {
	const forestCapacity = 200
	fullForest, err := mtrie.NewForest(forestCapacity, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)
	plForest, err := payloadless.NewForest(forestCapacity, &metrics.NoopCollector{}, nil)
	require.NoError(t, err)

	fullRoot := fullForest.GetEmptyRootHash()
	plRoot := plForest.GetEmptyRootHash()

	for round := 0; round < 12; round++ {
		paths := make([]ledger.Path, 0, 25)
		payloads := make([]ledger.Payload, 0, 25)
		for i := 0; i < 25; i++ {
			var p ledger.Path
			_, err := rand.Read(p[:])
			require.NoError(t, err)
			paths = append(paths, p)
			payloads = append(payloads, *testutils.RandomPayload(10, 80))
		}

		fullUpdate := &ledger.TrieUpdate{
			RootHash: fullRoot,
			Paths:    paths,
			Payloads: toPayloadPtrs(payloads),
		}
		plUpdate := &ledger.TrieUpdate{
			RootHash: plRoot,
			Paths:    paths,
			Payloads: toPayloadPtrs(payloads),
		}

		fullRoot, err = fullForest.Update(fullUpdate)
		require.NoError(t, err)
		plRoot, err = plForest.Update(plUpdate)
		require.NoError(t, err)

		require.Equal(t, fullRoot, plRoot, "root hashes diverged at round %d", round)
	}
}

// toPayloadPtrs converts a slice of payloads to a slice of payload pointers.
func toPayloadPtrs(payloads []ledger.Payload) []*ledger.Payload {
	ptrs := make([]*ledger.Payload, len(payloads))
	for i := range payloads {
		ptrs[i] = &payloads[i]
	}
	return ptrs
}

// TestConvertCheckpointV6ToV7_Deterministic checks that converting the same V6
// checkpoint twice (into separate output directories) yields byte-identical
// V7 part files. This protects against accidental non-determinism in the
// converter (e.g. map iteration leaking into the on-disk order).
func TestConvertCheckpointV6ToV7_Deterministic(t *testing.T) {
	unittest.RunWithTempDir(t, func(srcDir string) {
		unittest.RunWithTempDir(t, func(dst1 string) {
			unittest.RunWithTempDir(t, func(dst2 string) {
				logger := zerolog.Nop()
				v6Tries := createMultipleRandomTries(t)
				v6Name := "checkpoint.00000010"
				require.NoError(t, StoreCheckpointV6Concurrently(v6Tries, srcDir, v6Name, logger))

				v7Name := v6Name + V7FileSuffix
				require.NoError(t, ConvertCheckpointV6ToV7(srcDir, v6Name, dst1, v7Name, logger, 16))
				require.NoError(t, ConvertCheckpointV6ToV7(srcDir, v6Name, dst2, v7Name, logger, 16))

				files1 := filePaths(dst1, v7Name, subtrieLevel)
				files2 := filePaths(dst2, v7Name, subtrieLevel)
				require.Equal(t, len(files1), len(files2))
				for i, f1 := range files1 {
					require.NoError(t, compareFiles(f1, files2[i]), "V7 part files differ at index %d", i)
				}
			})
		})
	})
}

// TestConvertCheckpointV6ToV7_IntermediateNWorker covers a worker count that is
// neither 1 nor subtrieCount, exercising the partial-pool path of the writer.
func TestConvertCheckpointV6ToV7_IntermediateNWorker(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		logger := zerolog.Nop()
		v6Tries := createMultipleRandomTries(t)
		v6Name := "checkpoint.00000011"
		require.NoError(t, StoreCheckpointV6Concurrently(v6Tries, dir, v6Name, logger))

		for _, nWorker := range []uint{2, 4, 8} {
			v7Name := fmt.Sprintf("%s.nw%d%s", v6Name, nWorker, V7FileSuffix)
			require.NoError(t, ConvertCheckpointV6ToV7(dir, v6Name, dir, v7Name, logger, nWorker))

			v7Tries, err := OpenAndReadCheckpointV7(dir, v7Name, logger)
			require.NoError(t, err)
			for i, v6 := range v6Tries {
				require.Equal(t, v6.RootHash(), v7Tries[i].RootHash(),
					"trie %d root hash mismatch at nWorker=%d", i, nWorker)
			}
		}
	})
}

// TestConvertCheckpointV6ToV7_MatchesDirectV7Write verifies that the V7 produced
// by the converter matches a V7 produced by writing the equivalent payloadless
// tries directly. This pins down the equivalence between "convert V6 then
// store" and "convert tries first then store directly".
func TestConvertCheckpointV6ToV7_MatchesDirectV7Write(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		logger := zerolog.Nop()
		v6Tries := createMultipleRandomTries(t)
		v6Name := "checkpoint.00000012"
		require.NoError(t, StoreCheckpointV6Concurrently(v6Tries, dir, v6Name, logger))

		// Path A: converter.
		convertedName := v6Name + ".converted" + V7FileSuffix
		require.NoError(t, ConvertCheckpointV6ToV7(dir, v6Name, dir, convertedName, logger, 16))

		// Path B: convert tries in-memory and write directly.
		v7Tries, err := FromV6Tries(v6Tries)
		require.NoError(t, err)
		directName := v6Name + ".direct" + V7FileSuffix
		require.NoError(t, StoreCheckpointV7Concurrently(v7Tries, dir, directName, logger))

		convertedFiles := filePaths(dir, convertedName, subtrieLevel)
		directFiles := filePaths(dir, directName, subtrieLevel)
		require.Equal(t, len(convertedFiles), len(directFiles))
		for i, cf := range convertedFiles {
			require.NoError(t, compareFiles(cf, directFiles[i]),
				"converter output differs from direct V7 write at part %d", i)
		}
	})
}

// TestConvertCheckpointV6ToV7_JunkInput verifies that a file that does not look
// like a V6 checkpoint surfaces an error rather than silently producing
// garbage.
func TestConvertCheckpointV6ToV7_JunkInput(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		logger := zerolog.Nop()
		v6Name := "checkpoint.00000013"
		junkPath := filepath.Join(dir, v6Name)
		require.NoError(t, writeBytes(junkPath, []byte("not a checkpoint header")))

		err := ConvertCheckpointV6ToV7(dir, v6Name, dir, v6Name+V7FileSuffix, logger, 16)
		require.Error(t, err, "junk V6 header file must be rejected")
	})
}

// writeBytes is a tiny helper for emitting junk test fixtures.
func writeBytes(filePath string, b []byte) error {
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = f.Write(b)
	return err
}
