package wal

import (
	"fmt"
	"os"
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
		require.NoError(t, ConvertCheckpointV6ToV7(dir, v6Name, dir, nonStreamName, logger, 16, false))

		// Path B: streaming converter.
		streamName := v6Name + ".stream" + V7FileSuffix
		require.NoError(t, ConvertCheckpointV6ToV7(dir, v6Name, dir, streamName, logger, 16, true))

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
		require.NoError(t, ConvertCheckpointV6ToV7(dir, v6Name, dir, v7Name, logger, 16, true))

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
			require.NoError(t, ConvertCheckpointV6ToV7(dir, v6Name, dir, v7Name, logger, nWorker, true))

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
		require.NoError(t, ConvertCheckpointV6ToV7(dir, v6Name, dir, v7Name, logger, 16, true))

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

		require.Error(t, ConvertCheckpointV6ToV7(dir, "x", dir, "out"+V7FileSuffix, logger, 0, true),
			"nWorker=0 must be rejected")
		require.Error(t, ConvertCheckpointV6ToV7(dir, "x", dir, "out"+V7FileSuffix, logger, 17, true),
			"nWorker > subtrieCount must be rejected")
		require.Error(t, ConvertCheckpointV6ToV7(dir, "missing", dir, "missing"+V7FileSuffix, logger, 4, true),
			"missing V6 input must be reported")

		v6Tries := createSimpleTrie(t)
		v6Name := "checkpoint.00000304"
		require.NoError(t, StoreCheckpointV6Concurrently(v6Tries, dir, v6Name, logger))

		require.Error(t, ConvertCheckpointV6ToV7(dir, v6Name, dir, "no-suffix", logger, 4, true),
			"output filename without V7 suffix must be rejected")

		v7Name := v6Name + V7FileSuffix
		require.NoError(t, ConvertCheckpointV6ToV7(dir, v6Name, dir, v7Name, logger, 4, true))
		require.Error(t, ConvertCheckpointV6ToV7(dir, v6Name, dir, v7Name, logger, 4, true),
			"second conversion to the same V7 output must be rejected")
	})
}

// TestConvertCheckpointV6ToV7_RejectedRerunKeepsOutput verifies that a conversion
// rejected because its output already exists leaves that output intact: the
// failure happens before anything is written, so the cleanup of partial output
// must not run and delete a previously converted checkpoint.
func TestConvertCheckpointV6ToV7_RejectedRerunKeepsOutput(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprintf("stream=%v", stream), func(t *testing.T) {
			unittest.RunWithTempDir(t, func(dir string) {
				logger := zerolog.Nop()
				v6Tries := createMultipleRandomTries(t)
				v6Name := "checkpoint.00000305"
				require.NoError(t, StoreCheckpointV6Concurrently(v6Tries, dir, v6Name, logger))

				v7Name := v6Name + V7FileSuffix
				require.NoError(t, ConvertCheckpointV6ToV7(dir, v6Name, dir, v7Name, logger, 16, stream))

				require.Error(t, ConvertCheckpointV6ToV7(dir, v6Name, dir, v7Name, logger, 16, stream),
					"second conversion to the same V7 output must be rejected")

				// the rejected re-run must not have touched the existing V7 checkpoint
				v7Tries, err := OpenAndReadCheckpointV7(dir, v7Name, logger)
				require.NoError(t, err, "existing V7 output must survive a rejected re-run")
				require.Equal(t, len(v6Tries), len(v7Tries))
				for i, v6 := range v6Tries {
					require.Equal(t, v6.RootHash(), v7Tries[i].RootHash(), "trie %d root hash mismatch", i)
				}
			})
		})
	}
}

// TestConvertCheckpointV6ToV7Stream_DetectsCorruptedInput verifies that the stream
// converter CRC-verifies the input bytes it converts: flipping a single byte of a
// V6 part file - leaving both stored checksums intact - must fail the conversion
// rather than produce a V7 checkpoint carrying corrupted data under a freshly
// computed, valid checksum.
func TestConvertCheckpointV6ToV7Stream_DetectsCorruptedInput(t *testing.T) {
	// index of the V6 part file to corrupt: the largest subtrie file, and the
	// top-trie file (always the (subtrieCount)-th part file)
	for _, partFile := range []string{"subtrie", "toptrie"} {
		t.Run(partFile, func(t *testing.T) {
			unittest.RunWithTempDir(t, func(dir string) {
				logger := zerolog.Nop()
				v6Tries := createMultipleRandomTries(t)
				v6Name := "checkpoint.00000306"
				require.NoError(t, StoreCheckpointV6Concurrently(v6Tries, dir, v6Name, logger))

				var path string
				if partFile == "toptrie" {
					path, _ = filePathTopTries(dir, v6Name)
				} else {
					path = largestSubTrieFilePath(t, dir, v6Name)
				}

				// flip the last byte of the file's content: it belongs to the last
				// encoded node (or trie root record) and precedes the footer and the
				// stored checksum, so both stored checksums remain unchanged
				footerSize := encNodeCountSize + crc32SumSize
				if partFile == "toptrie" {
					footerSize = encNodeCountSize + encTrieCountSize + crc32SumSize
				}
				corruptByteAt(t, path, -(int64(footerSize) + 1))

				v7Name := v6Name + V7FileSuffix
				err := ConvertCheckpointV6ToV7(dir, v6Name, dir, v7Name, logger, 16, true)
				require.Error(t, err, "corrupted V6 input must be detected")
				require.Contains(t, err.Error(), "invalid checksum")

				// no V7 output must be left behind
				files, err := findCheckpointPartFiles(dir, v7Name)
				require.NoError(t, err)
				require.Empty(t, files, "failed conversion must not leave output files behind")
			})
		})
	}
}

// largestSubTrieFilePath returns the path of the V6 subtrie part file with the most
// content, i.e. the one guaranteed to hold encoded nodes.
func largestSubTrieFilePath(t *testing.T, dir string, fileName string) string {
	var largestPath string
	var largestSize int64
	for i := 0; i < subtrieCount; i++ {
		path, _, err := filePathSubTries(dir, fileName, i)
		require.NoError(t, err)
		info, err := os.Stat(path)
		require.NoError(t, err)
		if info.Size() > largestSize {
			largestSize, largestPath = info.Size(), path
		}
	}
	require.NotEmpty(t, largestPath)
	return largestPath
}

// corruptByteAt flips all bits of a single byte of the given file. A negative
// offset is interpreted relative to the end of the file.
func corruptByteAt(t *testing.T, path string, offset int64) {
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	if offset < 0 {
		offset += int64(len(content))
	}
	require.GreaterOrEqual(t, offset, int64(0))
	require.Less(t, offset, int64(len(content)))
	content[offset] ^= 0xFF
	require.NoError(t, os.WriteFile(path, content, 0644))
}
