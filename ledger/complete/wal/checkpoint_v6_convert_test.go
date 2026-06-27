package wal

import (
	"fmt"
	"os"
	"path"
	"testing"

	prometheusWAL "github.com/onflow/wal/wal"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestConvertCheckpointV7ToV6_RoundTrip builds a previous full V6 checkpoint plus
// WAL segments carrying later updates, converts a V7 checkpoint of the resulting
// forest back into a V6 checkpoint, and verifies the reconstruction is correct.
//
// Correctness is checked with VerifyCheckpointHashes, which re-derives each V6
// leaf hash from its reconstructed payload and compares it to the stored node
// hash. Because the converter carries node hashes over verbatim, this is the
// check that actually proves every payload was sourced correctly (a wrong payload
// with a carried-over hash would fail re-derivation). Trie root hashes and a
// register spot-check provide additional confidence.
func TestConvertCheckpointV7ToV6_RoundTrip(t *testing.T) {
	for _, nWorker := range []uint{1, 4, 16} {
		t.Run(fmt.Sprintf("nWorker=%d", nWorker), func(t *testing.T) {
			unittest.RunWithTempDir(t, func(dir string) {
				logger := zerolog.Nop()

				allTries, lastTrie, lastPaths, lastValues := setupV7ToV6Scenario(t, dir, logger)

				outDir := path.Join(dir, "out")
				require.NoError(t, os.MkdirAll(outDir, 0755))

				v7Name := "checkpoint.00000005.v7"
				outName := "checkpoint.00000005"

				first, last, err := prometheusWAL.Segments(dir)
				require.NoError(t, err)
				require.GreaterOrEqual(t, first, 0, "expected WAL segments to exist")

				// Convert with explicit overrides: previous checkpoint M=0 and the
				// full available WAL range. Over-scanning the WAL is safe — the pool
				// is keyed by leaf hash, so extra entries never produce wrong matches.
				require.NoError(t, ConvertCheckpointV7ToV6(
					dir, v7Name, dir, 0, first, last, outDir, outName, logger, nWorker))

				// The reconstructed V6 leaf hashes must re-derive from the payloads.
				require.NoError(t, VerifyCheckpointHashes(logger, outDir, outName, nWorker))

				// Root hashes and trie count must match the source forest.
				reconstructed, err := OpenAndReadCheckpointV6(outDir, outName, logger)
				require.NoError(t, err)
				require.Equal(t, len(allTries), len(reconstructed))
				for i, expected := range allTries {
					require.Equal(t, expected.RootHash(), reconstructed[i].RootHash(),
						"trie %d root hash mismatch", i)
				}

				// Spot-check that the actual register values were recovered for the
				// latest trie (the one that mixes previous-checkpoint and WAL sources).
				reconLast := reconstructed[len(reconstructed)-1]
				require.Equal(t, lastTrie.RootHash(), reconLast.RootHash())
				expectedByPath := make(map[ledger.Path]ledger.Value, len(lastPaths))
				for i, p := range lastPaths {
					expectedByPath[p] = lastValues[i]
				}
				// UnsafeRead permutes its input in place and aligns results to the
				// permuted order, so compare against the (post-permutation) paths.
				readPaths := append([]ledger.Path{}, lastPaths...)
				got := reconLast.UnsafeRead(readPaths)
				require.Len(t, got, len(readPaths))
				for i := range readPaths {
					require.Equal(t, expectedByPath[readPaths[i]], got[i].Value(),
						"register value mismatch at path %x", readPaths[i])
				}
			})
		})
	}
}

// setupV7ToV6Scenario writes, into dir:
//   - a previous full V6 checkpoint "checkpoint.00000000" (state after update u0),
//   - WAL segments carrying two later updates u1 (with overwrites and new
//     registers) and u2 (with overwrites and a deletion), and
//   - a V7 checkpoint "checkpoint.00000005.v7" of the forest holding all three
//     trie states.
//
// It returns all three trie states, the final trie, and the final trie's paths
// and expected values for a register spot-check.
func setupV7ToV6Scenario(t *testing.T, dir string, logger zerolog.Logger) (
	allTries []*trie.MTrie, lastTrie *trie.MTrie, lastPaths []ledger.Path, lastValues []ledger.Value,
) {
	// u0: initial state -> trie0. Stored only in the previous full checkpoint.
	pathsA, payloadsA := randNPathPayloads(50)
	trie0, _, err := trie.NewTrieWithUpdatedRegisters(trie.NewEmptyMTrie(), pathsA, payloadsA, true)
	require.NoError(t, err)

	require.NoError(t, StoreCheckpointV6Concurrently([]*trie.MTrie{trie0}, dir, "checkpoint.00000000", logger))

	// u1: overwrite the first 10 of A with new values, plus 20 brand-new registers.
	// Overwrites keep each path's original key and only change the value, honoring
	// the MTrie invariant that a path's key never changes (in Flow, path = hash(key)).
	overwrites1 := overwriteValues(t, payloadsA[:10])
	pathsB, payloadsB := randNPathPayloads(20)
	u1Paths := append(append([]ledger.Path{}, pathsA[:10]...), pathsB...)
	u1Payloads := append(append([]ledger.Payload{}, overwrites1...), payloadsB...)
	trie1, _, err := trie.NewTrieWithUpdatedRegisters(trie0, u1Paths, u1Payloads, true)
	require.NoError(t, err)

	// u2: overwrite the first 5 of B, and delete one of A (empty payload).
	overwrites2 := overwriteValues(t, payloadsB[:5])
	u2Paths := append(append([]ledger.Path{}, pathsB[:5]...), pathsA[10])
	u2Payloads := append(append([]ledger.Payload{}, overwrites2...), *ledger.EmptyPayload())
	trie2, _, err := trie.NewTrieWithUpdatedRegisters(trie1, u2Paths, u2Payloads, true)
	require.NoError(t, err)

	// Record u1 and u2 into the WAL (u0 is intentionally NOT recorded — its values
	// must come from the previous checkpoint).
	recordWAL, err := NewDiskWAL(logger, nil, metrics.NewNoopCollector(), dir, 10, pathByteSize, segmentSize)
	require.NoError(t, err)
	_, _, err = recordWAL.RecordUpdate(&ledger.TrieUpdate{
		RootHash: trie0.RootHash(), Paths: u1Paths, Payloads: toPayloadPtrs(u1Payloads)})
	require.NoError(t, err)
	_, _, err = recordWAL.RecordUpdate(&ledger.TrieUpdate{
		RootHash: trie1.RootHash(), Paths: u2Paths, Payloads: toPayloadPtrs(u2Payloads)})
	require.NoError(t, err)
	<-recordWAL.Done()

	// V7 checkpoint of the forest holding all three trie states.
	allTries = []*trie.MTrie{trie0, trie1, trie2}
	v7Tries, err := FromV6Tries(allTries)
	require.NoError(t, err)
	require.NoError(t, StoreCheckpointV7Concurrently(v7Tries, dir, "checkpoint.00000005.v7", logger))

	// For the spot-check: the surviving B registers in trie2 and their values.
	lastPaths = pathsB
	lastValues = make([]ledger.Value, len(pathsB))
	for i := 0; i < 5; i++ {
		lastValues[i] = overwrites2[i].Value()
	}
	for i := 5; i < len(pathsB); i++ {
		lastValues[i] = payloadsB[i].Value()
	}
	return allTries, trie2, lastPaths, lastValues
}

// overwriteValues returns new payloads that keep each original payload's key but
// replace its value with a fresh random value, honoring the MTrie invariant that
// a path's key never changes across updates.
func overwriteValues(t *testing.T, originals []ledger.Payload) []ledger.Payload {
	_, randoms := randNPathPayloads(len(originals))
	out := make([]ledger.Payload, len(originals))
	for i, orig := range originals {
		key, err := orig.Key()
		require.NoError(t, err)
		out[i] = *ledger.NewPayload(key, randoms[i].Value())
	}
	return out
}

// TestConvertCheckpointV7ToV6_TopTrieLeaf exercises reconstruction of a leaf that
// lives in the top-trie part file — a register compactified above the subtrie
// split. A single-register trie's root is exactly such a leaf, so this
// deterministically pins the buildTopTriePayloadPool path (which is otherwise only
// hit by chance on larger random tries).
func TestConvertCheckpointV7ToV6_TopTrieLeaf(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		logger := zerolog.Nop()

		paths, payloads := randNPathPayloads(1)
		single, _, err := trie.NewTrieWithUpdatedRegisters(trie.NewEmptyMTrie(), paths, payloads, true)
		require.NoError(t, err)

		// The register's payload is sourced from the previous checkpoint; no WAL needed.
		require.NoError(t, StoreCheckpointV6Concurrently([]*trie.MTrie{single}, dir, "checkpoint.00000000", logger))
		v7Tries, err := FromV6Tries([]*trie.MTrie{single})
		require.NoError(t, err)
		require.NoError(t, StoreCheckpointV7Concurrently(v7Tries, dir, "checkpoint.00000005.v7", logger))

		outDir := path.Join(dir, "out")
		require.NoError(t, os.MkdirAll(outDir, 0755))

		// Empty WAL range (from > to): the payload comes from the previous checkpoint's top-trie.
		require.NoError(t, ConvertCheckpointV7ToV6(
			dir, "checkpoint.00000005.v7", dir, 0, 1, 0, outDir, "checkpoint.00000005", logger, 1))

		require.NoError(t, VerifyCheckpointHashes(logger, outDir, "checkpoint.00000005", 1))
		recon, err := OpenAndReadCheckpointV6(outDir, "checkpoint.00000005", logger)
		require.NoError(t, err)
		require.Len(t, recon, 1)
		require.Equal(t, single.RootHash(), recon[0].RootHash())
	})
}

// TestConvertCheckpointV7ToV6_AutoDiscoverPrev verifies that resolvePrevCheckpoint
// selects the latest V6 checkpoint strictly below the V7 checkpoint number, and
// errors when none qualifies.
func TestConvertCheckpointV7ToV6_AutoDiscoverPrev(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		logger := zerolog.Nop()
		tries := createSimpleTrie(t)
		for _, num := range []int{3, 7} {
			require.NoError(t, StoreCheckpointV6Concurrently(tries, dir, NumberToFilename(num), logger))
		}

		got, err := resolvePrevCheckpoint(dir, 10, -1)
		require.NoError(t, err)
		require.Equal(t, 7, got)

		got, err = resolvePrevCheckpoint(dir, 5, -1)
		require.NoError(t, err)
		require.Equal(t, 3, got)

		_, err = resolvePrevCheckpoint(dir, 2, -1)
		require.Error(t, err, "no checkpoint below 2 exists")

		// Override is honored and must be below N.
		got, err = resolvePrevCheckpoint(dir, 10, 3)
		require.NoError(t, err)
		require.Equal(t, 3, got)
		_, err = resolvePrevCheckpoint(dir, 5, 5)
		require.Error(t, err, "override must be < N")
	})
}

// TestConvertCheckpointV7ToV6_Validation covers argument and filename validation.
func TestConvertCheckpointV7ToV6_Validation(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		logger := zerolog.Nop()

		require.Error(t, ConvertCheckpointV7ToV6(dir, "x.v7", dir, -1, -1, -1, dir, "out", logger, 0),
			"nWorker=0 must be rejected")
		require.Error(t, ConvertCheckpointV7ToV6(dir, "x.v7", dir, -1, -1, -1, dir, "out", logger, 17),
			"nWorker > subtrieCount must be rejected")
		require.Error(t, ConvertCheckpointV7ToV6(dir, "x.v7", dir, -1, -1, -1, dir, "out"+V7FileSuffix, logger, 4),
			"output filename with V7 suffix must be rejected")
		require.Error(t, ConvertCheckpointV7ToV6(dir, "missing.v7", dir, -1, -1, -1, dir, "out", logger, 4),
			"missing V7 input must be reported")
	})
}

// TestRequireV6Filename checks the filename guard.
func TestRequireV6Filename(t *testing.T) {
	require.Error(t, requireV6Filename(""))
	require.Error(t, requireV6Filename("checkpoint.00000005"+V7FileSuffix))
	require.NoError(t, requireV6Filename("checkpoint.00000005"))
}
