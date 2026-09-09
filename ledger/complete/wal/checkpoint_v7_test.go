package wal

import (
	"crypto/rand"
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/testutils"
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

// TestReadCheckpointV7RootHash verifies that [ReadTriesRootHashV7] returns each
// stored trie's root hash without decoding the full payload.
func TestReadCheckpointV7RootHash(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		tries := createSimplePayloadlessTrie(t)
		fileName := "checkpoint-v7-readroot"
		logger := zerolog.Nop()
		require.NoErrorf(t, StoreCheckpointV7Concurrently(tries, dir, fileName, logger), "fail to store checkpoint")

		trieRoots, err := ReadTriesRootHashV7(logger, dir, fileName)
		require.NoError(t, err)
		require.Equal(t, len(tries), len(trieRoots))
		for i, root := range trieRoots {
			require.Equal(t, tries[i].RootHash(), root)
		}
	})
}

// TestReadCheckpointV7RootHashMulti covers the multi-trie / multi-subtrie path
// of [ReadTriesRootHashV7], ensuring tail-seek arithmetic holds when triesCount > 1.
func TestReadCheckpointV7RootHashMulti(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		tries := createMultiplePayloadlessTries(t)
		fileName := "checkpoint-v7-readroot-multi"
		logger := zerolog.Nop()
		require.NoErrorf(t, StoreCheckpointV7Concurrently(tries, dir, fileName, logger), "fail to store checkpoint")

		trieRoots, err := ReadTriesRootHashV7(logger, dir, fileName)
		require.NoError(t, err)
		require.Equal(t, len(tries), len(trieRoots))
		for i, root := range trieRoots {
			require.Equal(t, tries[i].RootHash(), root)
		}
	})
}

// TestReadCheckpointV7RootHashValidateChecksum corrupts the top-trie file's CRC32
// trailer and verifies [ReadTriesRootHashV7] surfaces the checksum mismatch.
func TestReadCheckpointV7RootHashValidateChecksum(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		tries := createSimplePayloadlessTrie(t)
		fileName := "checkpoint-v7-bad-checksum"
		logger := zerolog.Nop()
		require.NoErrorf(t, StoreCheckpointV7Concurrently(tries, dir, fileName, logger), "fail to store checkpoint")

		topTrieFilePath, _ := filePathTopTries(dir, fileName)
		file, err := os.OpenFile(topTrieFilePath, os.O_RDWR, 0644)
		require.NoError(t, err)

		fileInfo, err := file.Stat()
		require.NoError(t, err)
		fileSize := fileInfo.Size()

		invalidSum := encodeCRC32Sum(10)
		_, err = file.WriteAt(invalidSum, fileSize-crc32SumSize)
		require.NoError(t, err)
		require.NoError(t, file.Close())

		_, err = ReadTriesRootHashV7(logger, dir, fileName)
		require.Error(t, err)
	})
}

// TestReadCheckpointV7RootHashRejectsV6 confirms that [ReadTriesRootHashV7]
// refuses a V6 checkpoint (version is checked before trie-record decoding).
func TestReadCheckpointV7RootHashRejectsV6(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		tries := createSimpleTrie(t)
		fileName := "checkpoint-v6-for-v7-reader"
		logger := zerolog.Nop()
		require.NoErrorf(t, StoreCheckpointV6Concurrently(tries, dir, fileName, logger), "fail to store V6 checkpoint")

		_, err := ReadTriesRootHashV7(logger, dir, fileName)
		require.Error(t, err, "V7 root-hash reader must reject a V6 checkpoint")
	})
}

// TestCheckpointHasRootHashV7Dispatch verifies the [CheckpointHasRootHash]
// dispatcher routes through [ReadTriesRootHashV7] when the filename ends in
// [V7FileSuffix].
func TestCheckpointHasRootHashV7Dispatch(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		tries := createMultiplePayloadlessTries(t)
		fileName := "checkpoint-v7-dispatch" + V7FileSuffix
		logger := zerolog.Nop()
		require.NoErrorf(t, StoreCheckpointV7Concurrently(tries, dir, fileName, logger), "fail to store checkpoint")

		trieRoots, err := ReadTriesRootHashV7(logger, dir, fileName)
		require.NoError(t, err)
		require.NotEmpty(t, trieRoots)
		for _, root := range trieRoots {
			require.NoError(t, CheckpointHasRootHash(logger, dir, fileName, root))
		}

		nonExist := ledger.RootHash(unittest.StateCommitmentFixture())
		require.Error(t, CheckpointHasRootHash(logger, dir, fileName, nonExist))
	})
}

// randomPayloadlessNode mirrors `randomNode` for the payloadless node type: a leaf
// node at height 256 with a random path and hash, and no leaf hash.
func randomPayloadlessNode() *payloadless.Node {
	var randomPath ledger.Path
	_, err := rand.Read(randomPath[:])
	if err != nil {
		panic("randomness failed")
	}

	var randomHashValue hash.Hash
	_, err = rand.Read(randomHashValue[:])
	if err != nil {
		panic("randomness failed")
	}

	return payloadless.NewNode(256, nil, nil, randomPath, nil, randomHashValue)
}

// TestGetPayloadlessNodesByIndex is the V7 analog of `TestGetNodesByIndex`: it checks that
// the index assigned to a node while writing resolves back to the same node while reading,
// across the subtrie groups and the top-level node slice.
func TestGetPayloadlessNodesByIndex(t *testing.T) {
	n := 10
	ns := make([]*payloadless.Node, n)
	for i := 0; i < n; i++ {
		ns[i] = randomPayloadlessNode()
	}
	subtrieNodes := [][]*payloadless.Node{
		{ns[0], ns[1]},
		{ns[2]},
		{},
		{},
	}
	topLevelNodes := []*payloadless.Node{nil, ns[3]}
	totalSubTrieNodeCount := computeTotalPayloadlessSubTrieNodeCount(subtrieNodes)

	for i := uint64(1); i <= 4; i++ {
		node, err := getPayloadlessNodeByIndex(subtrieNodes, totalSubTrieNodeCount, topLevelNodes, i)
		require.NoError(t, err, "cannot get node by index", i)
		require.Same(t, ns[i-1], node, "got wrong node by index %v", i)
	}

	// index 0 is the nil sentinel
	nilNode, err := getPayloadlessNodeByIndex(subtrieNodes, totalSubTrieNodeCount, topLevelNodes, 0)
	require.NoError(t, err)
	require.Nil(t, nilNode)

	// an index past the top-level nodes is an error rather than a panic
	_, err = getPayloadlessNodeByIndex(subtrieNodes, totalSubTrieNodeCount, topLevelNodes, totalSubTrieNodeCount+10)
	require.Error(t, err)
}

// TestEncodeSubTrieV7 is the V7 analog of `TestEncodeSubTrie`: it stores each subtrie group
// to its own part file and verifies that every root is reachable under the index that
// `storeCheckpointSubTrieV7` reported for it.
func TestEncodeSubTrieV7(t *testing.T) {
	file := "checkpoint" + V7FileSuffix
	logger := zerolog.Nop()
	tries := createMultiplePayloadlessTries(t)
	estimatedSubtrieNodeCount := estimatePayloadlessSubtrieNodeCount(tries[0])
	subtrieRoots := createPayloadlessSubTrieRoots(tries)

	for index, roots := range subtrieRoots {
		unittest.RunWithTempDir(t, func(dir string) {
			uniqueIndices, nodeCount, checksum, err := storeCheckpointSubTrieV7(
				index, roots, estimatedSubtrieNodeCount, dir, file, logger)
			require.NoError(t, err)

			// subtrie roots might have duplicates, that's why they are grouped and each
			// group is stored in a different part file in order to deduplicate. The
			// returned uniqueIndices contains the index for each unique root. To verify
			// that, build uniqueRoots first, then verify no unique root is missing from
			// the uniqueIndices.
			uniqueRoots := make(map[*payloadless.Node]struct{})
			for _, root := range roots {
				uniqueRoots[root] = struct{}{}
			}

			// each root should be included in the uniqueIndices
			for _, root := range roots {
				_, ok := uniqueIndices[root]
				require.True(t, ok, "each root should be included in the uniqueIndices")
			}

			if len(uniqueIndices) > 1 {
				require.Len(t, uniqueIndices, len(uniqueRoots),
					"uniqueIndices should include all roots")
			}

			logger.Info().Msgf("payloadless sub trie checkpoint stored, uniqueIndices: %v, node count: %v, checksum: %v",
				uniqueIndices, nodeCount, checksum)

			// all the nodes
			nodes, err := readCheckpointSubTrieV7(dir, file, index, checksum, logger)
			require.NoError(t, err)

			for _, root := range roots {
				if root == nil {
					continue
				}
				index := uniqueIndices[root]
				require.Equal(t, root.Hash(), nodes[index-1].Hash(), // -1 because readCheckpointSubTrieV7 returns nodes[1:]
					"readCheckpointSubTrieV7 should return nodes where the root should be found "+
						"by the index specified by the uniqueIndices returned by storeCheckpointSubTrieV7")
			}
		})
	}
}

// TestCannotStoreTwiceV7 is the V7 analog of `TestCannotStoreTwice`: writing a checkpoint
// must never clobber part files already on disk under the same name.
func TestCannotStoreTwiceV7(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		tries := createSimplePayloadlessTrie(t)
		fileName := "checkpoint" + V7FileSuffix
		logger := zerolog.Nop()
		require.NoErrorf(t, StoreCheckpointV7Concurrently(tries, dir, fileName, logger), "fail to store checkpoint")
		// checkpoint already exists, can't store again
		require.Error(t, StoreCheckpointV7Concurrently(tries, dir, fileName, logger))
	})
}
