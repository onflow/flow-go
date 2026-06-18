package wal

import (
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestConvertCheckpointV6ToV7Stream_MatchesNonStream verifies that the streaming
// converter produces byte-identical V7 part files to the in-memory
// converter. Both preserve the V6 on-disk node ordering and use the same leaf
// projection and encoding, so their output must match exactly.
func TestConvertCheckpointV6ToV7Stream_MatchesNonStream(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		logger := zerolog.Nop()
		v6Tries := createMultipleRandomTries(t)
		v6Name := "checkpoint.00000300"
		require.NoError(t, StoreCheckpointV6Concurrently(v6Tries, dir, v6Name, logger))

		// Path A: in-memory converter.
		nonStreamName := v6Name + ".nonstream" + V7FileSuffix
		require.NoError(t, ConvertCheckpointV6ToV7(dir, v6Name, dir, nonStreamName, logger, 16))

		// Path B: streaming converter.
		streamName := v6Name + ".stream" + V7FileSuffix
		require.NoError(t, ConvertCheckpointV6ToV7Stream(dir, v6Name, dir, streamName, logger, 16))

		nonStreamFiles := filePaths(dir, nonStreamName, subtrieLevel)
		streamFiles := filePaths(dir, streamName, subtrieLevel)
		require.Equal(t, len(nonStreamFiles), len(streamFiles))
		for i, nf := range nonStreamFiles {
			require.NoError(t, compareFiles(nf, streamFiles[i]),
				"stream converter output differs from non-stream at part %d", i)
		}
	})
}

// TestConvertCheckpointV6ToV7Stream_PreservesRootHashes writes a V6 checkpoint,
// runs the stream converter, then reads the V7 result back and verifies every
// trie root hash matches.
func TestConvertCheckpointV6ToV7Stream_PreservesRootHashes(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		logger := zerolog.Nop()
		v6Tries := createMultipleRandomTries(t)
		v6Name := "checkpoint.00000301"
		require.NoError(t, StoreCheckpointV6Concurrently(v6Tries, dir, v6Name, logger))

		v7Name := v6Name + V7FileSuffix
		require.NoError(t, ConvertCheckpointV6ToV7Stream(dir, v6Name, dir, v7Name, logger, 16))

		v7Tries, err := OpenAndReadCheckpointV7(dir, v7Name, logger)
		require.NoError(t, err)
		require.Equal(t, len(v6Tries), len(v7Tries))
		for i, v6 := range v6Tries {
			require.Equal(t, v6.RootHash(), v7Tries[i].RootHash(), "trie %d root hash mismatch", i)
		}
	})
}

// TestConvertCheckpointV6ToV7Stream_NWorkerVariants covers the minimum, an
// intermediate, and the maximum worker counts.
func TestConvertCheckpointV6ToV7Stream_NWorkerVariants(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		logger := zerolog.Nop()
		v6Tries := createMultipleRandomTries(t)
		v6Name := "checkpoint.00000302"
		require.NoError(t, StoreCheckpointV6Concurrently(v6Tries, dir, v6Name, logger))

		for _, nWorker := range []uint{1, 3, 16} {
			v7Name := fmt.Sprintf("%s.nw%d%s", v6Name, nWorker, V7FileSuffix)
			require.NoError(t, ConvertCheckpointV6ToV7Stream(dir, v6Name, dir, v7Name, logger, nWorker))

			v7Tries, err := OpenAndReadCheckpointV7(dir, v7Name, logger)
			require.NoError(t, err)
			for i, v6 := range v6Tries {
				require.Equal(t, v6.RootHash(), v7Tries[i].RootHash(),
					"trie %d root hash mismatch at nWorker=%d", i, nWorker)
			}
		}
	})
}

// TestConvertCheckpointV6ToV7Stream_EmptyTrie verifies the stream converter handles
// an empty-trie checkpoint.
func TestConvertCheckpointV6ToV7Stream_EmptyTrie(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		logger := zerolog.Nop()
		v6Tries := []*trie.MTrie{trie.NewEmptyMTrie()}
		v6Name := "checkpoint.00000303"
		require.NoError(t, StoreCheckpointV6Concurrently(v6Tries, dir, v6Name, logger))

		v7Name := v6Name + V7FileSuffix
		require.NoError(t, ConvertCheckpointV6ToV7Stream(dir, v6Name, dir, v7Name, logger, 16))

		v7Tries, err := OpenAndReadCheckpointV7(dir, v7Name, logger)
		require.NoError(t, err)
		require.Len(t, v7Tries, 1)
		require.True(t, v7Tries[0].IsEmpty())
	})
}

// TestConvertCheckpointV6ToV7Stream_Validation verifies argument and filename
// validation: invalid worker counts, a non-V7 output filename, refusing to
// clobber an existing output, and a missing V6 input.
func TestConvertCheckpointV6ToV7Stream_Validation(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		logger := zerolog.Nop()

		require.Error(t, ConvertCheckpointV6ToV7Stream(dir, "x", dir, "out"+V7FileSuffix, logger, 0),
			"nWorker=0 must be rejected")
		require.Error(t, ConvertCheckpointV6ToV7Stream(dir, "x", dir, "out"+V7FileSuffix, logger, 17),
			"nWorker > subtrieCount must be rejected")
		require.Error(t, ConvertCheckpointV6ToV7Stream(dir, "missing", dir, "missing"+V7FileSuffix, logger, 4),
			"missing V6 input must be reported")

		v6Tries := createSimpleTrie(t)
		v6Name := "checkpoint.00000304"
		require.NoError(t, StoreCheckpointV6Concurrently(v6Tries, dir, v6Name, logger))

		require.Error(t, ConvertCheckpointV6ToV7Stream(dir, v6Name, dir, "no-suffix", logger, 4),
			"output filename without V7 suffix must be rejected")

		v7Name := v6Name + V7FileSuffix
		require.NoError(t, ConvertCheckpointV6ToV7Stream(dir, v6Name, dir, v7Name, logger, 4))
		require.Error(t, ConvertCheckpointV6ToV7Stream(dir, v6Name, dir, v7Name, logger, 4),
			"second conversion to the same V7 output must be rejected")
	})
}
