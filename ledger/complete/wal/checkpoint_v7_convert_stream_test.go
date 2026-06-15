package wal

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestConvertCheckpointV6ToV7Stream_MatchesNonStream verifies that the streaming
// converter produces byte-identical V7 part files to the in-memory
// converter. Both preserve the V6 on-disk node ordering and use the same leaf
// projection and encoding, so their output must match exactly. The check is run
// with leaf-hash verification both off and on, since verification must not alter
// the output bytes.
func TestConvertCheckpointV6ToV7Stream_MatchesNonStream(t *testing.T) {
	for _, verify := range []bool{false, true} {
		t.Run(fmt.Sprintf("verifyLeafHash=%v", verify), func(t *testing.T) {
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
				require.NoError(t, ConvertCheckpointV6ToV7Stream(dir, v6Name, dir, streamName, logger, 16, verify))

				nonStreamFiles := filePaths(dir, nonStreamName, subtrieLevel)
				streamFiles := filePaths(dir, streamName, subtrieLevel)
				require.Equal(t, len(nonStreamFiles), len(streamFiles))
				for i, nf := range nonStreamFiles {
					require.NoError(t, compareFiles(nf, streamFiles[i]),
						"stream converter output differs from non-stream at part %d", i)
				}
			})
		})
	}
}

// TestConvertCheckpointV6ToV7Stream_PreservesRootHashes writes a V6 checkpoint,
// runs the stream converter (with leaf-hash verification enabled), then reads the
// V7 result back and verifies every trie root hash matches.
func TestConvertCheckpointV6ToV7Stream_PreservesRootHashes(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		logger := zerolog.Nop()
		v6Tries := createMultipleRandomTries(t)
		v6Name := "checkpoint.00000301"
		require.NoError(t, StoreCheckpointV6Concurrently(v6Tries, dir, v6Name, logger))

		v7Name := v6Name + V7FileSuffix
		require.NoError(t, ConvertCheckpointV6ToV7Stream(dir, v6Name, dir, v7Name, logger, 16, true))

		v7Tries, err := OpenAndReadCheckpointV7(dir, v7Name, logger)
		require.NoError(t, err)
		require.Equal(t, len(v6Tries), len(v7Tries))
		for i, v6 := range v6Tries {
			require.Equal(t, v6.RootHash(), v7Tries[i].RootHash(), "trie %d root hash mismatch", i)
		}
	})
}

// TestConvertCheckpointV6ToV7Stream_NWorkerVariants covers the minimum, an
// intermediate, and the maximum worker counts, with leaf-hash verification on.
func TestConvertCheckpointV6ToV7Stream_NWorkerVariants(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		logger := zerolog.Nop()
		v6Tries := createMultipleRandomTries(t)
		v6Name := "checkpoint.00000302"
		require.NoError(t, StoreCheckpointV6Concurrently(v6Tries, dir, v6Name, logger))

		for _, nWorker := range []uint{1, 3, 16} {
			v7Name := fmt.Sprintf("%s.nw%d%s", v6Name, nWorker, V7FileSuffix)
			require.NoError(t, ConvertCheckpointV6ToV7Stream(dir, v6Name, dir, v7Name, logger, nWorker, true))

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
		require.NoError(t, ConvertCheckpointV6ToV7Stream(dir, v6Name, dir, v7Name, logger, 16, true))

		v7Tries, err := OpenAndReadCheckpointV7(dir, v7Name, logger)
		require.NoError(t, err)
		require.Len(t, v7Tries, 1)
		require.True(t, v7Tries[0].IsEmpty())
	})
}

// TestV6ToV7NodeConverter_VerifyLeafHash confirms that the per-leaf verification
// path actually detects a tampered leaf. A leaf whose stored V6 node hash no
// longer matches its (path, value) is rejected when verification is on, and is
// silently converted when it is off (the streaming path otherwise re-derives the
// leaf hash from the payload and ignores the stored node hash).
func TestV6ToV7NodeConverter_VerifyLeafHash(t *testing.T) {
	// A single-register trie compactifies to a leaf root, giving a real V6 leaf
	// node with a correct node hash.
	emptyTrie := trie.NewEmptyMTrie()
	p := testutils.PathByUint8(7)
	v := testutils.LightPayload8('A', 'a')
	updated, _, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, []ledger.Path{p}, []ledger.Payload{*v}, true)
	require.NoError(t, err)
	leaf := updated.RootNode()
	require.True(t, leaf.IsLeaf(), "test setup expects a compactified leaf root")

	scratch := make([]byte, 1024*4)
	shared := flattener.EncodeNode(leaf, 0, 0, scratch)
	encoded := make([]byte, len(shared)) // copy: EncodeNode shares the scratch buffer
	copy(encoded, shared)

	// A correct leaf converts cleanly with verification enabled.
	var out bytes.Buffer
	require.NoError(t, newV6ToV7NodeConverter(true).convertNode(bytes.NewReader(encoded), &out))

	// Corrupt the last byte of the stored node hash (offset within the fixed
	// prefix: type(1) + height(2) + hash(32)).
	corrupt := make([]byte, len(encoded))
	copy(corrupt, encoded)
	corrupt[fixedNodePrefixSize-1] ^= 0xFF

	// With verification on, the stored-vs-derived hash mismatch is detected.
	var outVerify bytes.Buffer
	err = newV6ToV7NodeConverter(true).convertNode(bytes.NewReader(corrupt), &outVerify)
	require.Error(t, err)
	require.Contains(t, err.Error(), "leaf hash verification failed")

	// With verification off, the corrupt node hash is ignored and conversion
	// succeeds (the V7 leaf hash is derived from the intact payload).
	var outNoVerify bytes.Buffer
	require.NoError(t, newV6ToV7NodeConverter(false).convertNode(bytes.NewReader(corrupt), &outNoVerify))
}

// TestConvertCheckpointV6ToV7Stream_Validation verifies argument and filename
// validation: invalid worker counts, a non-V7 output filename, refusing to
// clobber an existing output, and a missing V6 input.
func TestConvertCheckpointV6ToV7Stream_Validation(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		logger := zerolog.Nop()

		require.Error(t, ConvertCheckpointV6ToV7Stream(dir, "x", dir, "out"+V7FileSuffix, logger, 0, false),
			"nWorker=0 must be rejected")
		require.Error(t, ConvertCheckpointV6ToV7Stream(dir, "x", dir, "out"+V7FileSuffix, logger, 17, false),
			"nWorker > subtrieCount must be rejected")
		require.Error(t, ConvertCheckpointV6ToV7Stream(dir, "missing", dir, "missing"+V7FileSuffix, logger, 4, false),
			"missing V6 input must be reported")

		v6Tries := createSimpleTrie(t)
		v6Name := "checkpoint.00000304"
		require.NoError(t, StoreCheckpointV6Concurrently(v6Tries, dir, v6Name, logger))

		require.Error(t, ConvertCheckpointV6ToV7Stream(dir, v6Name, dir, "no-suffix", logger, 4, false),
			"output filename without V7 suffix must be rejected")

		v7Name := v6Name + V7FileSuffix
		require.NoError(t, ConvertCheckpointV6ToV7Stream(dir, v6Name, dir, v7Name, logger, 4, false))
		require.Error(t, ConvertCheckpointV6ToV7Stream(dir, v6Name, dir, v7Name, logger, 4, false),
			"second conversion to the same V7 output must be rejected")
	})
}
