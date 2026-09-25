package wal

import (
	"context"
	"sort"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestOpenAndReadLeafNodesFromCheckpointV6Concurrently checks that reading a checkpoint with
// several workers reads the same leaf nodes as reading it with a single worker.
func TestOpenAndReadLeafNodesFromCheckpointV6Concurrently(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		tries := createMultipleRandomTries(t)
		checkpointTrie := tries[len(tries)-1]
		fileName := "checkpoint"
		logger := zerolog.Nop()
		require.NoError(t, StoreCheckpointV6Concurrently([]*trie.MTrie{checkpointTrie}, dir, fileName, logger))

		// readPaths returns the paths of all leaf nodes of the checkpoint, sorted, and checks
		// that the leaf nodes are pushed in batches of at most LeafNodeBatchSize
		readPaths := func(workerCount int) []string {
			leafNodeBatches := make(chan []*LeafNode, 16)
			readErrCh := make(chan error, 1)
			go func() {
				readErrCh <- OpenAndReadLeafNodesFromCheckpointV6Concurrently(
					context.Background(), leafNodeBatches, dir, fileName, checkpointTrie.RootHash(), workerCount, logger)
			}()

			paths := make([]string, 0)
			for batch := range leafNodeBatches {
				require.LessOrEqual(t, len(batch), LeafNodeBatchSize)
				for _, leafNode := range batch {
					paths = append(paths, string(leafNode.Path[:]))
				}
			}
			require.NoError(t, <-readErrCh)

			sort.Strings(paths)
			return paths
		}

		expected := readPaths(1)
		require.Len(t, expected, len(checkpointTrie.AllPayloads()))

		// 2 and 32 exceed and stay below/above the number of part files of a checkpoint
		for _, workerCount := range []int{2, 16, 32} {
			require.Equal(t, expected, readPaths(workerCount), "worker count %d", workerCount)
		}
	})
}

// TestOpenAndReadLeafNodesFromCheckpointV6Concurrently_Cancelled checks that reading returns
// the context's error instead of blocking when the context is cancelled and nobody consumes
// the channel.
func TestOpenAndReadLeafNodesFromCheckpointV6Concurrently_Cancelled(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		tries := createMultipleRandomTries(t)
		checkpointTrie := tries[len(tries)-1]
		fileName := "checkpoint"
		logger := zerolog.Nop()
		require.NoError(t, StoreCheckpointV6Concurrently([]*trie.MTrie{checkpointTrie}, dir, fileName, logger))

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		leafNodeBatches := make(chan []*LeafNode, 1)
		err := OpenAndReadLeafNodesFromCheckpointV6Concurrently(
			ctx, leafNodeBatches, dir, fileName, checkpointTrie.RootHash(), 16, logger)
		require.ErrorIs(t, err, context.Canceled)

		// the channel is closed even though the read was cancelled; batches that were pushed
		// before the cancellation are still buffered in the channel
		for range leafNodeBatches {
		}
		_, ok := <-leafNodeBatches
		require.False(t, ok)
	})
}

// TestOpenAndReadLeafNodesFromCheckpointV6Concurrently_InvalidWorkerCount checks that a
// worker count below one is rejected.
func TestOpenAndReadLeafNodesFromCheckpointV6Concurrently_InvalidWorkerCount(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		leafNodeBatches := make(chan []*LeafNode, 1)
		err := OpenAndReadLeafNodesFromCheckpointV6Concurrently(
			context.Background(), leafNodeBatches, dir, "checkpoint", ledger.RootHash{}, 0, zerolog.Nop())
		require.ErrorContains(t, err, "worker count must be at least 1")
	})
}
