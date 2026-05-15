package wal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
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
