package wal

import (
	"crypto/rand"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/ledger/complete/mtrie"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestVersionV7(t *testing.T) {
	m, v, err := decodeVersion(encodeVersion(MagicBytesCheckpointHeader, VersionV7))
	require.NoError(t, err)
	require.Equal(t, MagicBytesCheckpointHeader, m)
	require.Equal(t, VersionV7, v)
}

// createSimplePayloadlessTrie creates a simple payloadless trie for testing
func createSimplePayloadlessTrie(t *testing.T) []*trie.MTrie {
	emptyTrie := trie.NewEmptyMTrieWithPayloadless(true)

	p1 := testutils.PathByUint8(0)
	v1 := testutils.LightPayload8('A', 'a')

	p2 := testutils.PathByUint8(1)
	v2 := testutils.LightPayload8('B', 'b')

	paths := []ledger.Path{p1, p2}
	payloads := []ledger.Payload{*v1, *v2}

	updatedTrie, _, err := trie.NewTrieWithUpdatedRegistersAndPayloadless(emptyTrie, paths, payloads, true, true)
	require.NoError(t, err)
	tries := []*trie.MTrie{updatedTrie}
	return tries
}

// createMultiplePayloadlessTries creates multiple payloadless tries for testing
func createMultiplePayloadlessTries(t *testing.T) []*trie.MTrie {
	tries := make([]*trie.MTrie, 0)
	activeTrie := trie.NewEmptyMTrieWithPayloadless(true)

	var err error
	for i := 0; i < 5; i++ {
		paths, payloads := randNPathPayloads(20)
		activeTrie, _, err = trie.NewTrieWithUpdatedRegistersAndPayloadless(activeTrie, paths, payloads, false, true)
		require.NoError(t, err, "update registers")
		tries = append(tries, activeTrie)
	}

	// trie must be deep enough to test the subtrie
	if !isTrieDeepEnough(activeTrie) {
		return createMultiplePayloadlessTries(t)
	}

	return tries
}

// requirePayloadlessTriesEqual compares two sets of payloadless tries
func requirePayloadlessTriesEqual(t *testing.T, tries1, tries2 []*trie.MTrie) {
	require.Equal(t, len(tries1), len(tries2), "tries have different length")
	for i, expect := range tries1 {
		actual := tries2[i]
		require.True(t, expect.Equals(actual), "%v-th trie is different", i)
		// Both should be payloadless
		require.True(t, expect.IsPayloadless(), "original trie should be payloadless")
		require.True(t, actual.IsPayloadless(), "loaded trie should be payloadless")
	}
}

func TestWriteAndReadCheckpointV7EmptyTrie(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		tries := []*trie.MTrie{trie.NewEmptyMTrieWithPayloadless(true)}
		fileName := "checkpoint-empty-trie-v7"
		logger := zerolog.Nop()
		require.NoErrorf(t, StoreCheckpointV7Concurrently(tries, dir, fileName, logger), "fail to store checkpoint")
		decoded, err := OpenAndReadCheckpointV7(dir, fileName, logger)
		require.NoErrorf(t, err, "fail to read checkpoint %v/%v", dir, fileName)
		requirePayloadlessTriesEqual(t, tries, decoded)
	})
}

func TestWriteAndReadCheckpointV7SimpleTrie(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		tries := createSimplePayloadlessTrie(t)
		fileName := "checkpoint-v7"
		logger := zerolog.Nop()
		require.NoErrorf(t, StoreCheckpointV7Concurrently(tries, dir, fileName, logger), "fail to store checkpoint")
		decoded, err := OpenAndReadCheckpointV7(dir, fileName, logger)
		require.NoErrorf(t, err, "fail to read checkpoint %v/%v", dir, fileName)
		requirePayloadlessTriesEqual(t, tries, decoded)
	})
}

func TestWriteAndReadCheckpointV7MultipleTries(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		tries := createMultiplePayloadlessTries(t)
		fileName := "checkpoint-multi-file-v7"
		logger := zerolog.Nop()
		require.NoErrorf(t, StoreCheckpointV7Concurrently(tries, dir, fileName, logger), "fail to store checkpoint")
		decoded, err := OpenAndReadCheckpointV7(dir, fileName, logger)
		require.NoErrorf(t, err, "fail to read checkpoint %v/%v", dir, fileName)
		requirePayloadlessTriesEqual(t, tries, decoded)
	})
}

// Test that V7 checkpoints are deterministic
func TestCheckpointV7IsDeterministic(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		tries := createMultiplePayloadlessTries(t)
		logger := zerolog.Nop()
		require.NoErrorf(t, StoreCheckpointV7Concurrently(tries, dir, "checkpoint1", logger), "fail to store checkpoint")
		require.NoErrorf(t, StoreCheckpointV7Concurrently(tries, dir, "checkpoint2", logger), "fail to store checkpoint")
		partFiles1 := filePaths(dir, "checkpoint1", subtrieLevel)
		partFiles2 := filePaths(dir, "checkpoint2", subtrieLevel)
		for i, partFile1 := range partFiles1 {
			partFile2 := partFiles2[i]
			require.NoError(t, compareFiles(
				partFile1, partFile2),
				"found difference in checkpoint files")
		}
	})
}

// Test that V7 can be loaded via LoadCheckpoint (generic loader)
func TestWriteAndReadCheckpointV7ViaGenericLoader(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		tries := createSimplePayloadlessTrie(t)
		fileName := "checkpoint-v7-generic"
		logger := zerolog.Nop()
		require.NoErrorf(t, StoreCheckpointV7Concurrently(tries, dir, fileName, logger), "fail to store checkpoint")
		// Use the generic LoadCheckpoint function that reads the version
		decoded, err := LoadCheckpoint(filepath.Join(dir, fileName), logger)
		require.NoErrorf(t, err, "fail to load checkpoint")
		requirePayloadlessTriesEqual(t, tries, decoded)
	})
}

// Test that V7 checkpoint stores correct root hash
func TestCheckpointV7RootHash(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		tries := createSimplePayloadlessTrie(t)
		fileName := "checkpoint-v7-roothash"
		logger := zerolog.Nop()
		require.NoErrorf(t, StoreCheckpointV7Concurrently(tries, dir, fileName, logger), "fail to store checkpoint")
		decoded, err := OpenAndReadCheckpointV7(dir, fileName, logger)
		require.NoErrorf(t, err, "fail to read checkpoint")
		// Verify root hash matches
		for i, t1 := range tries {
			require.Equal(t, t1.RootHash(), decoded[i].RootHash(), "root hash mismatch at index %d", i)
		}
	})
}

// Test that old code cannot read V7 checkpoint (version check)
func TestV7CheckpointVersionMismatch(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		tries := createSimplePayloadlessTrie(t)
		fileName := "checkpoint-v7-version"
		logger := zerolog.Nop()
		require.NoErrorf(t, StoreCheckpointV7Concurrently(tries, dir, fileName, logger), "fail to store checkpoint")
		// Try to read with V6 reader - should fail due to version mismatch
		_, err := OpenAndReadCheckpointV6(dir, fileName, logger)
		require.Error(t, err, "V6 reader should fail on V7 checkpoint")
	})
}

// Test that V7 reader cannot read V6 checkpoint
func TestV6CheckpointVersionMismatchV7Reader(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		// Create regular (non-payloadless) tries
		tries := createSimpleTrie(t)
		fileName := "checkpoint-v6"
		logger := zerolog.Nop()
		require.NoErrorf(t, StoreCheckpointV6Concurrently(tries, dir, fileName, logger), "fail to store checkpoint")
		// Try to read with V7 reader - should fail due to version mismatch
		_, err := OpenAndReadCheckpointV7(dir, fileName, logger)
		require.Error(t, err, "V7 reader should fail on V6 checkpoint")
	})
}

// Test single-threaded V7 writer
func TestWriteAndReadCheckpointV7SingleThread(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		tries := createSimplePayloadlessTrie(t)
		fileName := "checkpoint-v7-single"
		logger := zerolog.Nop()
		require.NoErrorf(t, StoreCheckpointV7SingleThread(tries, dir, fileName, logger), "fail to store checkpoint")
		decoded, err := OpenAndReadCheckpointV7(dir, fileName, logger)
		require.NoErrorf(t, err, "fail to read checkpoint")
		requirePayloadlessTriesEqual(t, tries, decoded)
	})
}

// Test that missing part files return appropriate error
func TestV7AllPartFileExist(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		for i := 0; i < 17; i++ {
			tries := createSimplePayloadlessTrie(t)
			fileName := "checkpoint_v7_missing_part"
			var fileToDelete string
			var err error
			if i == 16 {
				fileToDelete, _ = filePathTopTries(dir, fileName)
			} else {
				fileToDelete, _, err = filePathSubTries(dir, fileName, i)
			}
			require.NoErrorf(t, err, "fail to find sub trie file path")

			logger := zerolog.Nop()
			require.NoErrorf(t, StoreCheckpointV7Concurrently(tries, dir, fileName, logger), "fail to store checkpoint")

			// delete i-th part file
			err = os.Remove(fileToDelete)
			require.NoError(t, err, "fail to remove part file")

			_, err = OpenAndReadCheckpointV7(dir, fileName, logger)
			require.ErrorIs(t, err, os.ErrNotExist, "wrong error type returned for missing file %d", i)

			// cleanup for next iteration
			deleteCheckpointFiles(dir, fileName)
		}
	})
}

// Test that payloadless trie values are 32-byte hashes
func TestV7PayloadlessTrieStoresHashes(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		tries := createSimplePayloadlessTrie(t)
		fileName := "checkpoint-v7-hashes"
		logger := zerolog.Nop()
		require.NoErrorf(t, StoreCheckpointV7Concurrently(tries, dir, fileName, logger), "fail to store checkpoint")
		decoded, err := OpenAndReadCheckpointV7(dir, fileName, logger)
		require.NoErrorf(t, err, "fail to read checkpoint")

		// Verify payloads in decoded tries are 32-byte hashes
		for _, tr := range decoded {
			require.True(t, tr.IsPayloadless(), "decoded trie should be payloadless")
			allPayloads := tr.AllPayloads()
			for _, payload := range allPayloads {
				if payload.Value().Size() > 0 {
					require.Equal(t, 32, payload.Value().Size(),
						"payloadless trie should store 32-byte hashes, got %d bytes", payload.Value().Size())
				}
			}
		}
	})
}

// OpenAndReadCheckpointV7 opens the checkpoint file and reads it with readCheckpointV7
func OpenAndReadCheckpointV7(dir string, fileName string, logger zerolog.Logger) (
	triesToReturn []*trie.MTrie,
	errToReturn error,
) {
	filepath := filePathCheckpointHeader(dir, fileName)
	errToReturn = withFile(logger, filepath, func(file *os.File) error {
		tries, err := readCheckpointV7(file, logger)
		if err != nil {
			return err
		}
		triesToReturn = tries
		return nil
	})
	return triesToReturn, errToReturn
}

// Tests for OpenAndReadAsPayloadlessTrie

// TestOpenAndReadAsPayloadlessTrieFromV7 verifies reading V7 checkpoint as payloadless
func TestOpenAndReadAsPayloadlessTrieFromV7(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		tries := createSimplePayloadlessTrie(t)
		fileName := "checkpoint-v7-payloadless"
		logger := zerolog.Nop()
		require.NoErrorf(t, StoreCheckpointV7Concurrently(tries, dir, fileName, logger), "fail to store checkpoint")

		// Read using OpenAndReadAsPayloadlessTrie
		decoded, err := OpenAndReadAsPayloadlessTrie(dir, fileName, logger)
		require.NoErrorf(t, err, "fail to read checkpoint as payloadless")
		requirePayloadlessTriesEqual(t, tries, decoded)
	})
}

// TestOpenAndReadAsPayloadlessTrieFromV6 verifies reading V6 checkpoint as payloadless
func TestOpenAndReadAsPayloadlessTrieFromV6(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		// Create a payloadless trie but store it as V6
		// This simulates a V6 checkpoint created from a payloadless forest
		tries := createSimplePayloadlessTrie(t)
		fileName := "checkpoint-v6-as-payloadless"
		logger := zerolog.Nop()

		// Store as V6 (note: the payload values are already hashes from the payloadless trie)
		require.NoErrorf(t, StoreCheckpointV6Concurrently(tries, dir, fileName, logger), "fail to store checkpoint")

		// Read using OpenAndReadAsPayloadlessTrie - should work and return payloadless tries
		decoded, err := OpenAndReadAsPayloadlessTrie(dir, fileName, logger)
		require.NoErrorf(t, err, "fail to read V6 checkpoint as payloadless")

		// Verify the decoded tries are marked as payloadless
		for i, tr := range decoded {
			require.True(t, tr.IsPayloadless(), "decoded trie %d should be payloadless", i)
		}

		// Verify root hashes match
		for i, orig := range tries {
			require.Equal(t, orig.RootHash(), decoded[i].RootHash(), "root hash mismatch at index %d", i)
		}
	})
}

// TestOpenAndReadAsPayloadlessTriePreservesRootHash verifies root hash is preserved
func TestOpenAndReadAsPayloadlessTriePreservesRootHash(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		// Create payloadless tries
		tries := createMultiplePayloadlessTries(t)
		fileName := "checkpoint-roothash-test"
		logger := zerolog.Nop()

		// Store as V6
		require.NoErrorf(t, StoreCheckpointV6Concurrently(tries, dir, fileName, logger), "fail to store checkpoint")

		// Read as payloadless
		decoded, err := OpenAndReadAsPayloadlessTrie(dir, fileName, logger)
		require.NoErrorf(t, err, "fail to read checkpoint")

		// Verify all root hashes match
		require.Equal(t, len(tries), len(decoded), "trie count mismatch")
		for i, orig := range tries {
			require.Equal(t, orig.RootHash(), decoded[i].RootHash(),
				"root hash mismatch at index %d: expected %s, got %s",
				i, orig.RootHash(), decoded[i].RootHash())
		}
	})
}

// TestOpenAndReadAsPayloadlessTrieV6MultipleTries tests reading multiple tries from V6
func TestOpenAndReadAsPayloadlessTrieV6MultipleTries(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		tries := createMultiplePayloadlessTries(t)
		fileName := "checkpoint-v6-multi-payloadless"
		logger := zerolog.Nop()

		require.NoErrorf(t, StoreCheckpointV6Concurrently(tries, dir, fileName, logger), "fail to store checkpoint")

		decoded, err := OpenAndReadAsPayloadlessTrie(dir, fileName, logger)
		require.NoErrorf(t, err, "fail to read V6 checkpoint as payloadless")

		require.Equal(t, len(tries), len(decoded), "trie count mismatch")
		for i, tr := range decoded {
			require.True(t, tr.IsPayloadless(), "decoded trie %d should be payloadless", i)
			require.Equal(t, tries[i].RootHash(), tr.RootHash(), "root hash mismatch at index %d", i)
		}
	})
}

// TestOpenAndReadAsPayloadlessTrieUnsupportedVersion tests error handling for unsupported versions
func TestOpenAndReadAsPayloadlessTrieUnsupportedVersion(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		// Create a V5 checkpoint
		tries := createSimpleTrie(t)
		fileName := "checkpoint-v5"
		logger := zerolog.Nop()
		require.NoErrorf(t, storeCheckpointV5(tries, dir, fileName, logger), "fail to store checkpoint")

		// Try to read as payloadless - should fail for V5
		_, err := OpenAndReadAsPayloadlessTrie(dir, fileName, logger)
		require.Error(t, err, "should fail for V5 checkpoint")
		require.Contains(t, err.Error(), "unsupported checkpoint version", "error should mention unsupported version")
	})
}

// TestOpenAndReadAsPayloadlessTriePayloadValues verifies payload values are 32-byte hashes
func TestOpenAndReadAsPayloadlessTriePayloadValues(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		tries := createSimplePayloadlessTrie(t)
		fileName := "checkpoint-payload-values"
		logger := zerolog.Nop()

		// Store as V6
		require.NoErrorf(t, StoreCheckpointV6Concurrently(tries, dir, fileName, logger), "fail to store checkpoint")

		// Read as payloadless
		decoded, err := OpenAndReadAsPayloadlessTrie(dir, fileName, logger)
		require.NoErrorf(t, err, "fail to read checkpoint")

		// Verify payload values are 32-byte hashes
		for _, tr := range decoded {
			allPayloads := tr.AllPayloads()
			for _, payload := range allPayloads {
				if payload.Value().Size() > 0 {
					require.Equal(t, 32, payload.Value().Size(),
						"payload value should be 32-byte hash, got %d bytes", payload.Value().Size())
				}
			}
		}
	})
}

// TestV6V7CheckpointConsistencyWithWALUpdates is a comprehensive test that verifies:
// 1. Path 1: Load V6 checkpoint (from payloadless forest) -> apply WAL updates with payloadless forest -> get root hash
// 2. Path 2: Load V6 as payloadless -> apply WAL updates -> should get same root hash -> export V7
// 3. Path 3: Load V6 as payloadless -> export V7 -> load V7 -> apply WAL updates -> should get same root hash -> export V7
// Path 2 and Path 3 V7 checkpoints should be identical
//
// IMPORTANT: The V6 checkpoint is created from a PAYLOADLESS forest, so the payload values
// stored in the checkpoint are already 32-byte hashes. This allows OpenAndReadAsPayloadlessTrie
// to correctly load it as a payloadless trie.
func TestV6V7CheckpointConsistencyWithWALUpdates(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		logger := zerolog.Nop()
		forestCapacity := 100

		// ============================================================
		// Step 1: Create initial V6 checkpoint from a PAYLOADLESS forest
		// This is the key - the V6 checkpoint will contain payload hashes as values
		// ============================================================
		initialPaths, initialPayloads := generateInitialData(t, 50)

		// Create a PAYLOADLESS forest and add initial data
		payloadlessForest, err := mtrie.NewForestWithPayloadless(forestCapacity, &metrics.NoopCollector{}, nil, true)
		require.NoError(t, err, "failed to create payloadless forest")

		initialUpdate := &ledger.TrieUpdate{
			RootHash: payloadlessForest.GetEmptyRootHash(),
			Paths:    initialPaths,
			Payloads: toPayloadPtrs(initialPayloads),
		}
		initialRootHash, err := payloadlessForest.Update(initialUpdate)
		require.NoError(t, err, "failed to apply initial update")

		// Store as V6 checkpoint (the values stored are payload hashes, not full payloads)
		v6CheckpointFile := "checkpoint-v6-initial"
		initialTries, err := payloadlessForest.GetTries()
		require.NoError(t, err, "failed to get tries from forest")
		// Only store the trie with initial data (not the empty trie)
		var trieToStore *trie.MTrie
		for _, tr := range initialTries {
			if tr.RootHash() == initialRootHash {
				trieToStore = tr
				break
			}
		}
		require.NotNil(t, trieToStore, "failed to find trie with initial root hash")
		require.True(t, trieToStore.IsPayloadless(), "initial trie should be payloadless")
		require.NoErrorf(t, StoreCheckpointV6SingleThread([]*trie.MTrie{trieToStore}, dir, v6CheckpointFile, logger),
			"fail to store V6 checkpoint")

		t.Logf("Initial V6 checkpoint created from payloadless forest with root hash: %s", initialRootHash)

		// ============================================================
		// Step 2: Generate WAL updates (add, remove, update operations)
		// ============================================================
		walUpdates := generateWALUpdates(t, initialPaths, initialPayloads, 30)
		t.Logf("Generated %d WAL updates", len(walUpdates))

		// ============================================================
		// Path 1: Load V6 as payloadless -> apply WAL updates -> get final root hash
		// This is the reference path that Path 2 and Path 3 should match
		// ============================================================
		path1Dir := path.Join(dir, "path1")
		require.NoError(t, os.MkdirAll(path1Dir, 0755))

		// Copy checkpoint to path1
		_, err = CopyCheckpointFile(v6CheckpointFile, dir, path1Dir)
		require.NoError(t, err, "failed to copy checkpoint to path1")

		// Load V6 checkpoint as payloadless
		path1Tries, err := OpenAndReadAsPayloadlessTrie(path1Dir, v6CheckpointFile, logger)
		require.NoError(t, err, "failed to load V6 checkpoint as payloadless for path1")
		require.Len(t, path1Tries, 1, "expected 1 trie in checkpoint")
		require.True(t, path1Tries[0].IsPayloadless(), "path1 trie should be payloadless")

		// Create payloadless forest
		path1Forest, err := mtrie.NewForestWithPayloadless(forestCapacity, &metrics.NoopCollector{}, nil, true)
		require.NoError(t, err)
		require.NoError(t, path1Forest.AddTries(path1Tries))

		// Apply WAL updates
		path1RootHash := path1Tries[0].RootHash()
		for i, update := range walUpdates {
			update.RootHash = path1RootHash
			path1RootHash, err = path1Forest.Update(update)
			require.NoError(t, err, "failed to apply WAL update %d in path1", i)
		}
		t.Logf("Path 1 final root hash: %s", path1RootHash)

		// ============================================================
		// Path 2: Load V6 as payloadless -> apply WAL updates -> export V7
		// ============================================================
		path2Dir := path.Join(dir, "path2")
		require.NoError(t, os.MkdirAll(path2Dir, 0755))

		// Copy checkpoint to path2
		_, err = CopyCheckpointFile(v6CheckpointFile, dir, path2Dir)
		require.NoError(t, err, "failed to copy checkpoint to path2")

		// Load V6 checkpoint as payloadless
		path2Tries, err := OpenAndReadAsPayloadlessTrie(path2Dir, v6CheckpointFile, logger)
		require.NoError(t, err, "failed to load V6 checkpoint as payloadless for path2")
		require.Len(t, path2Tries, 1, "expected 1 trie in checkpoint")
		require.True(t, path2Tries[0].IsPayloadless(), "path2 trie should be payloadless")

		// Create payloadless forest
		path2Forest, err := mtrie.NewForestWithPayloadless(forestCapacity, &metrics.NoopCollector{}, nil, true)
		require.NoError(t, err)
		require.NoError(t, path2Forest.AddTries(path2Tries))

		// Apply WAL updates
		path2RootHash := path2Tries[0].RootHash()
		for i, update := range walUpdates {
			update.RootHash = path2RootHash
			path2RootHash, err = path2Forest.Update(update)
			require.NoError(t, err, "failed to apply WAL update %d in path2", i)
		}
		t.Logf("Path 2 final root hash: %s", path2RootHash)

		// Verify root hash matches path1
		require.Equal(t, path1RootHash, path2RootHash,
			"Path 2 root hash should match Path 1 root hash")

		// Export to V7 checkpoint
		v7CheckpointPath2 := "checkpoint-v7-path2"
		path2FinalTrie, err := path2Forest.GetTrie(path2RootHash)
		require.NoError(t, err, "failed to get final trie from path2 forest")
		require.NoErrorf(t, StoreCheckpointV7SingleThread([]*trie.MTrie{path2FinalTrie}, path2Dir, v7CheckpointPath2, logger),
			"fail to store V7 checkpoint for path2")

		// ============================================================
		// Path 3: Load V6 as payloadless -> export V7 -> load V7 -> apply WAL updates -> export V7
		// ============================================================
		path3Dir := path.Join(dir, "path3")
		require.NoError(t, os.MkdirAll(path3Dir, 0755))

		// Copy checkpoint to path3
		_, err = CopyCheckpointFile(v6CheckpointFile, dir, path3Dir)
		require.NoError(t, err, "failed to copy checkpoint to path3")

		// Load V6 checkpoint as payloadless
		path3Tries, err := OpenAndReadAsPayloadlessTrie(path3Dir, v6CheckpointFile, logger)
		require.NoError(t, err, "failed to load V6 checkpoint as payloadless for path3")
		require.Len(t, path3Tries, 1, "expected 1 trie in checkpoint")

		// Export to intermediate V7 checkpoint
		v7CheckpointIntermediate := "checkpoint-v7-intermediate"
		require.NoErrorf(t, StoreCheckpointV7SingleThread(path3Tries, path3Dir, v7CheckpointIntermediate, logger),
			"fail to store intermediate V7 checkpoint for path3")

		// Delete the V6 checkpoint files to ensure we're loading from V7
		deleteCheckpointFiles(path3Dir, v6CheckpointFile)

		// Load the intermediate V7 checkpoint
		path3TriesFromV7, err := OpenAndReadCheckpointV7(path3Dir, v7CheckpointIntermediate, logger)
		require.NoError(t, err, "failed to load intermediate V7 checkpoint for path3")
		require.Len(t, path3TriesFromV7, 1, "expected 1 trie in V7 checkpoint")
		require.True(t, path3TriesFromV7[0].IsPayloadless(), "path3 trie from V7 should be payloadless")

		// Verify intermediate root hash matches
		require.Equal(t, path2Tries[0].RootHash(), path3TriesFromV7[0].RootHash(),
			"Intermediate V7 checkpoint root hash should match original")

		// Create payloadless forest from V7 checkpoint
		path3Forest, err := mtrie.NewForestWithPayloadless(forestCapacity, &metrics.NoopCollector{}, nil, true)
		require.NoError(t, err)
		require.NoError(t, path3Forest.AddTries(path3TriesFromV7))

		// Apply WAL updates
		path3RootHash := path3TriesFromV7[0].RootHash()
		for i, update := range walUpdates {
			update.RootHash = path3RootHash
			path3RootHash, err = path3Forest.Update(update)
			require.NoError(t, err, "failed to apply WAL update %d in path3", i)
		}
		t.Logf("Path 3 final root hash: %s", path3RootHash)

		// Verify root hash matches path1 and path2
		require.Equal(t, path1RootHash, path3RootHash,
			"Path 3 root hash should match Path 1 root hash")

		// Export to V7 checkpoint
		v7CheckpointPath3 := "checkpoint-v7-path3"
		path3FinalTrie, err := path3Forest.GetTrie(path3RootHash)
		require.NoError(t, err, "failed to get final trie from path3 forest")
		require.NoErrorf(t, StoreCheckpointV7SingleThread([]*trie.MTrie{path3FinalTrie}, path3Dir, v7CheckpointPath3, logger),
			"fail to store V7 checkpoint for path3")

		// ============================================================
		// Verify Path 2 and Path 3 V7 checkpoints are identical
		// ============================================================
		path2V7Files := filePaths(path2Dir, v7CheckpointPath2, subtrieLevel)
		path3V7Files := filePaths(path3Dir, v7CheckpointPath3, subtrieLevel)

		require.Equal(t, len(path2V7Files), len(path3V7Files),
			"Path 2 and Path 3 V7 checkpoints should have same number of files")

		for i, path2File := range path2V7Files {
			path3File := path3V7Files[i]
			err := compareFiles(path2File, path3File)
			require.NoError(t, err,
				"Path 2 and Path 3 V7 checkpoint files should be identical: %s vs %s", path2File, path3File)
		}

		t.Log("SUCCESS: All paths produce identical results!")
		t.Logf("  - Path 1 (V6 -> payloadless -> apply WAL): root hash = %s", path1RootHash)
		t.Logf("  - Path 2 (V6 -> payloadless -> apply WAL -> V7): root hash = %s", path2RootHash)
		t.Logf("  - Path 3 (V6 -> payloadless -> V7 -> load -> apply WAL -> V7): root hash = %s", path3RootHash)
		t.Log("  - Path 2 and Path 3 V7 checkpoints are byte-for-byte identical")
	})
}

// generateInitialData creates initial paths and payloads for the test
func generateInitialData(t *testing.T, count int) ([]ledger.Path, []ledger.Payload) {
	paths := make([]ledger.Path, count)
	payloads := make([]ledger.Payload, count)

	for i := 0; i < count; i++ {
		var p ledger.Path
		_, err := rand.Read(p[:])
		require.NoError(t, err)
		paths[i] = p

		payload := testutils.RandomPayload(10, 100)
		payloads[i] = *payload
	}

	return paths, payloads
}

// generateWALUpdates creates a series of updates including add, remove, and update operations
func generateWALUpdates(t *testing.T, existingPaths []ledger.Path, existingPayloads []ledger.Payload, count int) []*ledger.TrieUpdate {
	updates := make([]*ledger.TrieUpdate, 0, count)

	// Track which paths exist and their current payloads
	pathState := make(map[ledger.Path]ledger.Payload)
	for i, p := range existingPaths {
		pathState[p] = existingPayloads[i]
	}

	existingPathsList := make([]ledger.Path, len(existingPaths))
	copy(existingPathsList, existingPaths)

	for i := 0; i < count; i++ {
		var updatePaths []ledger.Path
		var updatePayloads []*ledger.Payload

		// Randomly choose operation type
		opType := i % 3
		numOps := 1 + (i % 5) // 1-5 operations per update

		for j := 0; j < numOps; j++ {
			switch opType {
			case 0: // Add new value
				var newPath ledger.Path
				_, err := rand.Read(newPath[:])
				require.NoError(t, err)

				// Make sure it's actually new
				if _, exists := pathState[newPath]; !exists {
					newPayload := testutils.RandomPayload(10, 100)
					updatePaths = append(updatePaths, newPath)
					updatePayloads = append(updatePayloads, newPayload)
					pathState[newPath] = *newPayload
					existingPathsList = append(existingPathsList, newPath)
				}

			case 1: // Remove existing value (set to empty payload)
				if len(existingPathsList) > 0 {
					// Pick a random existing path
					idx := j % len(existingPathsList)
					pathToRemove := existingPathsList[idx]

					if _, exists := pathState[pathToRemove]; exists {
						emptyPayload := ledger.EmptyPayload()
						updatePaths = append(updatePaths, pathToRemove)
						updatePayloads = append(updatePayloads, emptyPayload)
						delete(pathState, pathToRemove)
					}
				}

			case 2: // Update existing value
				if len(existingPathsList) > 0 {
					// Pick a random existing path
					idx := j % len(existingPathsList)
					pathToUpdate := existingPathsList[idx]

					if _, exists := pathState[pathToUpdate]; exists {
						newPayload := testutils.RandomPayload(10, 100)
						updatePaths = append(updatePaths, pathToUpdate)
						updatePayloads = append(updatePayloads, newPayload)
						pathState[pathToUpdate] = *newPayload
					}
				}
			}
		}

		if len(updatePaths) > 0 {
			update := &ledger.TrieUpdate{
				Paths:    updatePaths,
				Payloads: updatePayloads,
			}
			updates = append(updates, update)
		}
	}

	return updates
}

// toPayloadPtrs converts a slice of payloads to a slice of payload pointers
func toPayloadPtrs(payloads []ledger.Payload) []*ledger.Payload {
	ptrs := make([]*ledger.Payload, len(payloads))
	for i := range payloads {
		ptrs[i] = &payloads[i]
	}
	return ptrs
}
