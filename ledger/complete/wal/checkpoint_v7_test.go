package wal

import (
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/ledger/complete/payloadless"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestVersionV7(t *testing.T) {
	m, v, err := decodeVersion(encodeVersion(MagicBytesCheckpointHeader, VersionV7))
	require.NoError(t, err)
	require.Equal(t, MagicBytesCheckpointHeader, m)
	require.Equal(t, VersionV7, v)
}

// createSimplePayloadlessTrie creates a single payloadless trie with two registers.
func createSimplePayloadlessTrie(t *testing.T) []*payloadless.MTrie {
	emptyTrie := payloadless.NewEmptyMTrie()

	p1 := testutils.PathByUint8(0)
	v1 := testutils.LightPayload8('A', 'a')

	p2 := testutils.PathByUint8(1)
	v2 := testutils.LightPayload8('B', 'b')

	paths := []ledger.Path{p1, p2}
	values := [][]byte{v1.Value(), v2.Value()}

	updatedTrie, _, err := payloadless.NewTrieWithUpdatedRegisters(emptyTrie, paths, values, true)
	require.NoError(t, err)
	return []*payloadless.MTrie{updatedTrie}
}

// createMultiplePayloadlessTries returns a chain of payloadless tries deep enough
// for the subtrie tests by stacking random updates.
func createMultiplePayloadlessTries(t *testing.T) []*payloadless.MTrie {
	tries := make([]*payloadless.MTrie, 0)
	activeTrie := payloadless.NewEmptyMTrie()

	var err error
	for i := 0; i < 5; i++ {
		paths, payloads := randNPathPayloads(20)
		values := payloadsToValues(payloads)
		activeTrie, _, err = payloadless.NewTrieWithUpdatedRegisters(activeTrie, paths, values, false)
		require.NoError(t, err, "update registers")
		tries = append(tries, activeTrie)
	}

	// trie must be deep enough to test the subtrie
	if !isTrieDeepEnoughPayloadless(activeTrie) {
		return createMultiplePayloadlessTries(t)
	}

	return tries
}

// isTrieDeepEnoughPayloadless mirrors the v6 helper for the payloadless trie type.
// It checks that every node at the subtrieLevel boundary is a non-leaf interim
// node, so subtrie-splitting paths in the encoder are exercised.
func isTrieDeepEnoughPayloadless(t *payloadless.MTrie) bool {
	nodes := getPayloadlessNodesAtLevel(t.RootNode(), subtrieLevel)
	for _, n := range nodes {
		if n == nil || n.IsLeaf() {
			return false
		}
	}
	return true
}

func payloadsToValues(payloads []ledger.Payload) [][]byte {
	values := make([][]byte, len(payloads))
	for i := range payloads {
		values[i] = payloads[i].Value()
	}
	return values
}

// requirePayloadlessTriesEqual compares two slices of payloadless tries by structural Equals.
func requirePayloadlessTriesEqual(t *testing.T, tries1, tries2 []*payloadless.MTrie) {
	require.Equal(t, len(tries1), len(tries2), "tries have different length")
	for i, expect := range tries1 {
		actual := tries2[i]
		require.True(t, expect.Equals(actual), "%v-th trie is different", i)
	}
}

func TestWriteAndReadCheckpointV7EmptyTrie(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		tries := []*payloadless.MTrie{payloadless.NewEmptyMTrie()}
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

// TestCheckpointV7IsDeterministic verifies that two calls to StoreCheckpointV7
// over the same tries produce byte-identical part files.
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

// TestCheckpointV7RootHash verifies that round-tripping a V7 checkpoint preserves the trie root hash.
func TestCheckpointV7RootHash(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		tries := createSimplePayloadlessTrie(t)
		fileName := "checkpoint-v7-roothash"
		logger := zerolog.Nop()
		require.NoErrorf(t, StoreCheckpointV7Concurrently(tries, dir, fileName, logger), "fail to store checkpoint")
		decoded, err := OpenAndReadCheckpointV7(dir, fileName, logger)
		require.NoErrorf(t, err, "fail to read checkpoint")
		for i, t1 := range tries {
			require.Equal(t, t1.RootHash(), decoded[i].RootHash(), "root hash mismatch at index %d", i)
		}
	})
}

// TestV7CheckpointVersionMismatch verifies the V6 reader rejects a V7 file.
func TestV7CheckpointVersionMismatch(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		tries := createSimplePayloadlessTrie(t)
		fileName := "checkpoint-v7-version"
		logger := zerolog.Nop()
		require.NoErrorf(t, StoreCheckpointV7Concurrently(tries, dir, fileName, logger), "fail to store checkpoint")
		_, err := OpenAndReadCheckpointV6(dir, fileName, logger)
		require.Error(t, err, "V6 reader should fail on V7 checkpoint")
	})
}

// TestV6CheckpointVersionMismatchV7Reader verifies the V7 reader rejects a V6 file.
func TestV6CheckpointVersionMismatchV7Reader(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		tries := createSimpleTrie(t)
		fileName := "checkpoint-v6"
		logger := zerolog.Nop()
		require.NoErrorf(t, StoreCheckpointV6Concurrently(tries, dir, fileName, logger), "fail to store checkpoint")
		_, err := OpenAndReadCheckpointV7(dir, fileName, logger)
		require.Error(t, err, "V7 reader should fail on V6 checkpoint")
	})
}

// TestWriteAndReadCheckpointV7SingleThread covers the single-threaded encoder path.
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

// TestV7AllPartFileExist verifies that a missing part file surfaces os.ErrNotExist.
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

			err = os.Remove(fileToDelete)
			require.NoError(t, err, "fail to remove part file")

			_, err = OpenAndReadCheckpointV7(dir, fileName, logger)
			require.ErrorIs(t, err, os.ErrNotExist, "wrong error type returned for missing file %d", i)

			require.NoError(t, deleteCheckpointFiles(dir, fileName))
		}
	})
}

// TestV7PayloadlessTrieStoresHashes verifies that the projected on-disk form
// stores 32-byte leaf hashes for every allocated register.
func TestV7PayloadlessTrieStoresHashes(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		tries := createSimplePayloadlessTrie(t)
		fileName := "checkpoint-v7-hashes"
		logger := zerolog.Nop()
		require.NoErrorf(t, StoreCheckpointV7Concurrently(tries, dir, fileName, logger), "fail to store checkpoint")
		decoded, err := OpenAndReadCheckpointV7(dir, fileName, logger)
		require.NoErrorf(t, err, "fail to read checkpoint")

		// Every leaf hash recovered from the decoded payloadless trie must be 32 bytes.
		for _, tr := range decoded {
			for _, lh := range tr.AllLeafHashes() {
				require.NotNil(t, lh, "decoded payloadless trie has nil leaf hash for an allocated register")
				require.Equal(t, hash.HashLen, len(lh), "leaf hash should be %d bytes, got %d", hash.HashLen, len(lh))
			}
		}
	})
}

// TestOpenAndReadCheckpointV7RejectsV6 verifies that the V7 reader refuses a V6
// checkpoint — version, not payload shape, is the gate.
func TestOpenAndReadCheckpointV7RejectsV6(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		tries := createSimpleTrie(t)
		fileName := "checkpoint-v6"
		logger := zerolog.Nop()
		require.NoErrorf(t, StoreCheckpointV6Concurrently(tries, dir, fileName, logger), "fail to store V6 checkpoint")

		_, err := OpenAndReadCheckpointV7(dir, fileName, logger)
		require.Error(t, err, "V7 reader must reject a V6 checkpoint")
	})
}

// TestOpenAndReadCheckpointV7RejectsV5 verifies that the V7 reader refuses a V5 checkpoint.
func TestOpenAndReadCheckpointV7RejectsV5(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		tries := createSimpleTrie(t)
		fileName := "checkpoint-v5"
		logger := zerolog.Nop()
		require.NoErrorf(t, storeCheckpointV5(tries, dir, fileName, logger), "fail to store V5 checkpoint")

		_, err := OpenAndReadCheckpointV7(dir, fileName, logger)
		require.Error(t, err, "V7 reader must reject a V5 checkpoint")
	})
}

// Ensure the trie package import is retained for converter use in helpers above.
var _ = trie.NewEmptyMTrie
