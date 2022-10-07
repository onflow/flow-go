package wal

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestVersion(t *testing.T) {
	m, v, err := decodeVersion(encodeVersion(MagicBytesCheckpointHeader, VersionV6))
	require.NoError(t, err)
	require.Equal(t, MagicBytesCheckpointHeader, m)
	require.Equal(t, VersionV6, v)
}

func TestSubtrieCount(t *testing.T) {
	l, err := decodeSubtrieCount(encodeSubtrieCount(subtrieCount))
	require.NoError(t, err)
	require.Equal(t, uint16(subtrieCount), l)
}

func TestCRC32SumEncoding(t *testing.T) {
	v := uint32(3)
	s, err := decodeCRC32Sum(encodeCRC32Sum(v))
	require.NoError(t, err)
	require.Equal(t, v, s)
}

func TestSubtrieFooterEncoding(t *testing.T) {
	v := uint64(100)
	s, err := decodeSubtrieFooter(encodeSubtrieFooter(v))
	require.NoError(t, err)
	require.Equal(t, v, s)
}

func TestFooterEncoding(t *testing.T) {
	n1, r1 := uint64(40), uint16(500)
	n2, r2, err := decodeFooter(encodeFooter(n1, r1))
	require.NoError(t, err)
	require.Equal(t, n1, n2)
	require.Equal(t, r1, r2)
}

func requireTriesEqual(t *testing.T, tries1, tries2 []*trie.MTrie) {
	require.Equal(t, len(tries1), len(tries2), "tries have different length")
	for i, expect := range tries1 {
		actual := tries2[i]
		require.True(t, expect.Equals(actual), "%v-th trie is different", i)
	}
}

func createSimpleTrie(t *testing.T) []*trie.MTrie {
	emptyTrie := trie.NewEmptyMTrie()

	p1 := testutils.PathByUint8(0)
	v1 := testutils.LightPayload8('A', 'a')

	p2 := testutils.PathByUint8(1)
	v2 := testutils.LightPayload8('B', 'b')

	paths := []ledger.Path{p1, p2}
	payloads := []ledger.Payload{*v1, *v2}

	updatedTrie, _, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths, payloads, true)
	require.NoError(t, err)
	tries := []*trie.MTrie{updatedTrie}
	return tries
}

func randPathPayload() (ledger.Path, ledger.Payload) {
	var path ledger.Path
	rand.Read(path[:])
	payload := testutils.RandomPayload(1, 100)
	return path, *payload
}

func randNPathPayloads(n int) ([]ledger.Path, []ledger.Payload) {
	paths := make([]ledger.Path, n)
	payloads := make([]ledger.Payload, n)
	for i := 0; i < n; i++ {
		path, payload := randPathPayload()
		paths[i] = path
		payloads[i] = payload
	}
	return paths, payloads
}

func createMultipleRandomTries(t *testing.T) []*trie.MTrie {
	tries := make([]*trie.MTrie, 0)
	activeTrie := trie.NewEmptyMTrie()

	// add tries with no shared paths
	for i := 0; i < 100; i++ {
		paths, payloads := randNPathPayloads(100)
		activeTrie, _, err := trie.NewTrieWithUpdatedRegisters(activeTrie, paths, payloads, false)
		require.NoError(t, err, "update registers")
		tries = append(tries, activeTrie)
	}

	// add trie with some shared path
	sharedPaths, payloads1 := randNPathPayloads(100)
	activeTrie, _, err := trie.NewTrieWithUpdatedRegisters(activeTrie, sharedPaths, payloads1, false)
	require.NoError(t, err, "update registers")
	tries = append(tries, activeTrie)

	_, payloads2 := randNPathPayloads(100)
	activeTrie, _, err = trie.NewTrieWithUpdatedRegisters(activeTrie, sharedPaths, payloads2, false)
	require.NoError(t, err, "update registers")
	tries = append(tries, activeTrie)

	return tries
}

func TestEncodeSubTrie(t *testing.T) {
	file := "checkpoint"
	logger := unittest.Logger()
	tries := createMultipleRandomTries(t)
	estimatedSubtrieNodeCount := estimateSubtrieNodeCount(tries[0])
	subtrieRoots := createSubTrieRoots(tries)

	for index, roots := range subtrieRoots {
		unittest.RunWithTempDir(t, func(dir string) {
			indices, nodeCount, checksum, err := storeCheckpointSubTrie(
				index, roots, estimatedSubtrieNodeCount, dir, file, &logger)
			require.NoError(t, err)

			if len(indices) > 1 {
				require.Len(t, indices, len(roots),
					fmt.Sprintf("indices should include all roots, indices[nil] %v, roots[0] %v", indices[nil], roots[0]))
			}
			// each root should be included in the indices
			for _, root := range roots {
				_, ok := indices[root]
				require.True(t, ok, "each root should be included in the indices")
			}

			logger.Info().Msgf("sub trie checkpoint stored, indices: %v, node count: %v, checksum: %v",
				indices, nodeCount, checksum)

			// all the nodes
			nodes, err := readCheckpointSubTrie(dir, file, index, checksum, &logger)
			require.NoError(t, err)

			for _, root := range roots {
				if root == nil {
					continue
				}
				index := indices[root]
				require.Equal(t, root.Hash(), nodes[index-1].Hash(), // -1 because readCheckpointSubTrie returns nodes[1:]
					"readCheckpointSubTrie should return nodes where the root should be found "+
						"by the index specified by the indices returned by storeCheckpointSubTrie")
			}
		})
	}
}

func randomNode() *node.Node {
	var randomPath ledger.Path
	rand.Read(randomPath[:])

	var randomHashValue hash.Hash
	rand.Read(randomHashValue[:])

	return node.NewNode(256, nil, nil, randomPath, nil, randomHashValue)
}
func TestGetNodesByIndex(t *testing.T) {
	n := 10
	ns := make([]*node.Node, n)
	for i := 0; i < n; i++ {
		ns[i] = randomNode()
	}
	subtrieNodes := [][]*node.Node{
		[]*node.Node{ns[0], ns[1]},
		[]*node.Node{ns[2]},
		[]*node.Node{},
		[]*node.Node{},
	}
	topLevelNodes := []*node.Node{nil, ns[3]}
	totalSubTrieNodeCount := computeTotalSubTrieNodeCount(subtrieNodes)

	for i := uint64(1); i <= 4; i++ {
		node, err := getNodeByIndex(subtrieNodes, totalSubTrieNodeCount, topLevelNodes, i)
		require.NoError(t, err, "cannot get node by index", i)
		require.Equal(t, ns[i-1], node, "got wrong node by index %v", i)
	}
}

func TestWriteAndReadCheckpointV6(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		tries := createSimpleTrie(t)
		fileName := "checkpoint"
		logger := unittest.Logger()
		require.NoErrorf(t, StoreCheckpointV6Concurrent(tries, dir, fileName, &logger), "fail to store checkpoint")
		decoded, err := OpenAndReadCheckpointV6(dir, fileName, &logger)
		require.NoErrorf(t, err, "fail to read checkpoint %v/%v", dir, fileName)
		requireTriesEqual(t, tries, decoded)
	})
}

func TestWriteAndReadCheckpointV6MultipleTries(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		tries := createMultipleRandomTries(t)
		fileName := "checkpoint-multi-file"
		logger := unittest.Logger()
		require.NoErrorf(t, StoreCheckpointV6Concurrent(tries, dir, fileName, &logger), "fail to store checkpoint")
		decoded, err := OpenAndReadCheckpointV6(dir, fileName, &logger)
		require.NoErrorf(t, err, "fail to read checkpoint %v/%v", dir, fileName)
		requireTriesEqual(t, tries, decoded)
	})
}

// test running checkpointing twice will produce the same checkpoint file
func TestCheckpointV6IsDeterminstic(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		tries := createMultipleRandomTries(t)
		logger := unittest.Logger()
		require.NoErrorf(t, StoreCheckpointV6Concurrent(tries, dir, "checkpoint1", &logger), "fail to store checkpoint")
		require.NoErrorf(t, StoreCheckpointV6Concurrent(tries, dir, "checkpoint2", &logger), "fail to store checkpoint")
		require.NoError(t, compareFiles(
			path.Join(dir, "checkpoint1"),
			path.Join(dir, "checkpoint2")),
			"found difference in checkpoint files")
	})
}

// compareFiles takes two files' full path, and read them bytes by bytes and compare if
// the two files are identical
// it returns nil if identical
// it returns error if there is difference
func compareFiles(file1, file2 string) error {
	closable1, err := os.Open(file1)
	if err != nil {
		return fmt.Errorf("could not open file 1 %v: %w", closable1, err)
	}
	defer func(f *os.File) {
		f.Close()
	}(closable1)

	closable2, err := os.Open(file1)
	if err != nil {
		return fmt.Errorf("could not open file 2 %v: %w", closable2, err)
	}
	defer func(f *os.File) {
		f.Close()
	}(closable2)

	reader1 := bufio.NewReaderSize(closable1, defaultBufioReadSize)
	reader2 := bufio.NewReaderSize(closable2, defaultBufioReadSize)

	buf1 := make([]byte, defaultBufioReadSize)
	buf2 := make([]byte, defaultBufioReadSize)
	for {
		_, err1 := reader1.Read(buf1)
		_, err2 := reader2.Read(buf2)
		if errors.Is(err1, io.EOF) && errors.Is(err2, io.EOF) {
			break
		}

		if err1 != nil {
			return err1
		}
		if err2 != nil {
			return err2
		}

		if !bytes.Equal(buf1, buf2) {
			return fmt.Errorf("bytes are different: %x, %x", buf1, buf2)
		}
	}

	return nil
}

func storeCheckpointV5(tries []*trie.MTrie, dir string, fileName string, logger *zerolog.Logger) error {
	closable, err := createWriterForCheckpointHeader(dir, fileName, logger)
	if err != nil {
		return fmt.Errorf("could not store checkpoint header: %w", err)
	}
	defer func() {
		closeErr := closable.Close()
		// Return close error if there isn't any prior error to return.
		if err == nil {
			err = closeErr
		}
	}()

	return StoreCheckpointV5(closable, tries...)
}

func TestWriteAndReadCheckpointV5(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		tries := createMultipleRandomTries(t)
		fileName := "checkpoint1"
		logger := unittest.Logger()

		require.NoErrorf(t, storeCheckpointV5(tries, dir, fileName, &logger), "fail to store checkpoint")
		decoded, err := LoadCheckpoint(filepath.Join(dir, fileName), &logger)
		require.NoErrorf(t, err, "fail to load checkpoint")
		requireTriesEqual(t, tries, decoded)
	})
}

// test that converting a v6 back to v5 would produce the same v5 checkpoint as
// producing directly to v5
func TestWriteAndReadCheckpointV6ThenBackToV5(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		tries := createMultipleRandomTries(t)
		logger := unittest.Logger()

		// store tries into v6 then read back, then store into v5
		require.NoErrorf(t, StoreCheckpointV6Concurrent(tries, dir, "checkpoint-v6", &logger), "fail to store checkpoint")
		decoded, err := OpenAndReadCheckpointV6(dir, "checkpoint-v6", &logger)
		require.NoErrorf(t, err, "fail to read checkpoint %v/checkpoint-v6", dir)
		require.NoErrorf(t, storeCheckpointV5(decoded, dir, "checkpoint-v6-v5", &logger), "fail to store checkpoint")

		// store tries directly into v5 checkpoint
		require.NoErrorf(t, storeCheckpointV5(tries, dir, "checkpoint-v5", &logger), "fail to store checkpoint")

		// compare the two v5 checkpoint files should be identical
		require.NoError(t, compareFiles(
			path.Join(dir, "checkpoint-v5"),
			path.Join(dir, "checkpoint-v6-v5")),
			"found difference in checkpoint files")
	})
}

func TestCleanupOnErrorIfNotExist(t *testing.T) {
	t.Run("works if temp files not exist", func(t *testing.T) {
		require.NoError(t, cleanupTempFiles("not-exist", "checkpoint-v6"))
	})

	// if it can clean up all files after successful storing, then it can
	// clean up if failed in middle.
	t.Run("clean up after finish storing files", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(dir string) {
			tries := createMultipleRandomTries(t)
			logger := unittest.Logger()

			// store tries into v6 then read back, then store into v5
			require.NoErrorf(t, StoreCheckpointV6Concurrent(tries, dir, "checkpoint-v6", &logger), "fail to store checkpoint")
			require.NoError(t, cleanupTempFiles(dir, "checkpoint-v6"))

			// verify all files are removed
			files := filePaths(dir, "checkpoint-v6", subtrieLevel)
			for _, file := range files {
				_, err := os.Stat(file)
				require.True(t, os.IsNotExist(err), err)
			}
		})
	})
}

// verify that if a part file is missing then os.ErrNotExist should return
func TestAllPartFileExist(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		for i := 0; i < 17; i++ {
			tries := createSimpleTrie(t)
			fileName := fmt.Sprintf("checkpoint_missing_part_file_%v", i)
			var fileToDelete string
			var err error
			if i == 16 {
				fileToDelete, _ = filePathTopTries(dir, fileName)
			} else {
				fileToDelete, _, err = filePathSubTries(dir, fileName, i)
			}
			require.NoErrorf(t, err, "fail to find sub trie file path")

			logger := unittest.Logger()
			require.NoErrorf(t, StoreCheckpointV6Concurrent(tries, dir, fileName, &logger), "fail to store checkpoint")

			// delete i-th part file, then the error should mention i-th file missing
			err = os.Remove(fileToDelete)
			require.NoError(t, err, "fail to remove part file")

			_, err = OpenAndReadCheckpointV6(dir, fileName, &logger)
			require.ErrorIs(t, err, os.ErrNotExist, "wrong error type returned")
		}
	})
}

// verify that can't store the same checkpoint file twice, because a checkpoint already exists
func TestCannotStoreTwice(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		tries := createSimpleTrie(t)
		fileName := "checkpoint"
		logger := unittest.Logger()
		require.NoErrorf(t, StoreCheckpointV6Concurrent(tries, dir, fileName, &logger), "fail to store checkpoint")
		// checkpoint already exist, can't store again
		require.Error(t, StoreCheckpointV6Concurrent(tries, dir, fileName, &logger))
	})
}
