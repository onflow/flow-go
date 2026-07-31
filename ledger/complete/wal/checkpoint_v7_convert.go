package wal

import (
	"fmt"
	"os"
	"path"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/ledger/complete/payloadless"
)

// FromV6LeafNode converts a V6 leaf [node.Node] into the equivalent V7
// (payloadless) [payloadless.Node]. The conversion preserves the node's
// path, height, and computed hash; the payload value is replaced by the
// height-0 leaf hash HashLeaf(path, value).
//
// For an unallocated leaf (empty or nil payload), the result is a payloadless
// leaf with leafHash == nil and the same default-for-height node hash.
//
// Expected error returns during normal operation:
//   - none — the only failure mode is passing an interim node, which is treated
//     as a programmer error rather than a benign error.
func FromV6LeafNode(v6 *node.Node) (*payloadless.Node, error) {
	if v6 == nil {
		return nil, fmt.Errorf("FromV6LeafNode: nil node")
	}
	if !v6.IsLeaf() {
		return nil, fmt.Errorf("FromV6LeafNode: node at height %d is not a leaf", v6.Height())
	}
	p := v6.Payload()
	if p == nil || p.IsEmpty() {
		// Unallocated leaf. Preserve the disk-stored hash explicitly via NewNode.
		return payloadless.NewNode(v6.Height(), nil, nil, *v6.Path(), nil, v6.Hash()), nil
	}
	leafHash := hash.HashLeaf(hash.Hash(*v6.Path()), p.Value())
	return payloadless.NewLeafWithHash(*v6.Path(), leafHash, v6.Height()), nil
}

// fromV6InterimNode converts a V6 interim [node.Node] into the equivalent V7
// interim [payloadless.Node] given the already-converted children. The interim
// hash is preserved verbatim so the resulting trie's root hash equals the V6
// root hash by induction.
func fromV6InterimNode(v6 *node.Node, lchild, rchild *payloadless.Node) *payloadless.Node {
	return payloadless.NewNode(v6.Height(), lchild, rchild, ledger.DummyPath, nil, v6.Hash())
}

// FromV6Trie converts a V6 [trie.MTrie] into the equivalent V7 (payloadless)
// [payloadless.MTrie]. Every node is converted via [FromV6LeafNode] (leaves)
// or fromV6InterimNode (interim), preserving the node hashes; consequently the
// resulting V7 trie has the same root hash as the input V6 trie.
//
// Shared sub-tries in the input (e.g. across a forest of related tries) are
// converted only once thanks to the visited-node memoization.
//
// No error returns are expected during normal operation.
func FromV6Trie(v6 *trie.MTrie) (*payloadless.MTrie, error) {
	if v6.IsEmpty() {
		return payloadless.NewEmptyMTrie(), nil
	}
	visited := make(map[*node.Node]*payloadless.Node)
	root, err := convertV6Subtree(v6.RootNode(), visited)
	if err != nil {
		return nil, err
	}
	return payloadless.NewMTrie(root, v6.AllocatedRegCount())
}

// convertV6Subtree converts an entire V6 subtree rooted at `n` and returns the
// equivalent V7 root. Shared sub-tries are memoized through `visited`.
func convertV6Subtree(n *node.Node, visited map[*node.Node]*payloadless.Node) (*payloadless.Node, error) {
	if n == nil {
		return nil, nil
	}
	if existing, ok := visited[n]; ok {
		return existing, nil
	}
	if n.IsLeaf() {
		converted, err := FromV6LeafNode(n)
		if err != nil {
			return nil, fmt.Errorf("could not convert leaf node: %w", err)
		}
		visited[n] = converted
		return converted, nil
	}
	lchild, err := convertV6Subtree(n.LeftChild(), visited)
	if err != nil {
		return nil, err
	}
	rchild, err := convertV6Subtree(n.RightChild(), visited)
	if err != nil {
		return nil, err
	}
	converted := fromV6InterimNode(n, lchild, rchild)
	visited[n] = converted
	return converted, nil
}

// FromV6Tries converts a slice of V6 tries to V7 tries, preserving root hashes.
// Sub-tries shared across multiple input tries are converted once.
//
// No error returns are expected during normal operation.
func FromV6Tries(v6Tries []*trie.MTrie) ([]*payloadless.MTrie, error) {
	visited := make(map[*node.Node]*payloadless.Node)
	out := make([]*payloadless.MTrie, len(v6Tries))
	for i, v6 := range v6Tries {
		if v6.IsEmpty() {
			out[i] = payloadless.NewEmptyMTrie()
			continue
		}
		root, err := convertV6Subtree(v6.RootNode(), visited)
		if err != nil {
			return nil, fmt.Errorf("could not convert V6 trie %d: %w", i, err)
		}
		v7, err := payloadless.NewMTrie(root, v6.AllocatedRegCount())
		if err != nil {
			return nil, fmt.Errorf("could not construct payloadless trie %d: %w", i, err)
		}
		out[i] = v7
	}
	return out, nil
}

// ConvertCheckpointV6ToV7 reads a V6 checkpoint at (inputDir, inputFileName),
// converts it to a V7 (payloadless) checkpoint, and writes it to
// (outputDir, outputFileName).
//
// Behavior:
//   - The input V6 part files (header + 17 part files) must all be present.
//   - The output filename must use the V7 suffix (e.g. "checkpoint.00000100.v7");
//     a missing or wrong suffix is rejected.
//   - No output file (including any part file) with the same name may already
//     exist; otherwise the call is rejected and the existing output is left
//     untouched.
//   - The conversion preserves trie root hashes: a V7 checkpoint round-tripped
//     through this function matches the V6 root hashes exactly.
//   - On any failure after the checks above, the partially written output is
//     removed.
//
// `stream` selects the conversion strategy:
//   - false: read the entire V6 forest into memory, convert it, and write the V7
//     checkpoint. Peak memory is approximately the sum of the V6 trie set and the
//     V7 trie set, so mainnet-scale checkpoints need a host with memory headroom.
//   - true: stream each part file node-by-node (see
//     [convertCheckpointV6ToV7Stream]). Peak memory is independent of checkpoint
//     size, at the cost of not re-deriving the trie root hashes from the
//     converted nodes.
//
// nWorker controls how many of the 16 subtrie part files are processed in
// parallel; valid range is [1, 16]. In the non-streaming mode, the V6 read step
// also reads the 16 subtrie part files concurrently using its own internal worker
// pool (this function does not gate that), so the total parallelism while
// running may exceed nWorker briefly during the read→write hand-off.
//
// Expected error returns during normal operation:
//   - none — all error returns indicate a malformed input, a clobbering output,
//     or a write failure, which are treated as exceptions.
func ConvertCheckpointV6ToV7(
	inputDir string,
	inputFileName string,
	outputDir string,
	outputFileName string,
	logger zerolog.Logger,
	nWorker uint,
	stream bool,
) error {
	subtrieChecksums, topTrieChecksum, err := validateV6ToV7Conversion(
		inputDir, inputFileName, outputDir, outputFileName, logger, nWorker)
	if err != nil {
		return err
	}

	logger.Info().
		Str("v6_dir", inputDir).
		Str("v6_file", inputFileName).
		Str("v7_dir", outputDir).
		Str("v7_file", outputFileName).
		Uint("nworker", nWorker).
		Bool("stream", stream).
		Msg("starting V6→V7 checkpoint conversion")

	if stream {
		err = convertCheckpointV6ToV7Stream(
			inputDir, inputFileName, outputDir, outputFileName, logger, nWorker, subtrieChecksums, topTrieChecksum)
	} else {
		err = convertCheckpointV6ToV7InMemory(inputDir, inputFileName, outputDir, outputFileName, logger, nWorker)
	}

	if err != nil {
		// validateV6ToV7Conversion established that no output file existed before this
		// call, so every file matching the output name now was written by this failed
		// call and is safe to remove.
		cleanupErr := deleteCheckpointFiles(outputDir, outputFileName)
		if cleanupErr != nil {
			return fmt.Errorf("fail to cleanup partially written output %s, after running into error: %w",
				cleanupErr, err)
		}
		return err
	}

	logger.Info().Msg("V6→V7 checkpoint conversion complete")
	return nil
}

// validateV6ToV7Conversion performs the pre-conversion checks shared by both
// conversion strategies and returns the per-subtrie checksums and the top-trie
// checksum recorded in the V6 checkpoint header.
//
// This function must run before any output file is created, and it must not
// create any itself: a failure here means this call wrote nothing, so the caller
// must not run output cleanup - which would delete a pre-existing V7 checkpoint
// belonging to a previous, successful conversion.
//
// No error returns are expected during normal operation.
func validateV6ToV7Conversion(
	inputDir string,
	inputFileName string,
	outputDir string,
	outputFileName string,
	logger zerolog.Logger,
	nWorker uint,
) ([]uint32, uint32, error) {
	if nWorker == 0 || nWorker > subtrieCount {
		return nil, 0, fmt.Errorf("invalid nWorker %v, valid range is [1, %v]", nWorker, subtrieCount)
	}

	// Reject obvious filename misuse so converted files can coexist with the V6 source.
	if err := requireV7Filename(outputFileName); err != nil {
		return nil, 0, err
	}

	// Validate V6 input exists (header + part files).
	v6Header := filePathCheckpointHeader(inputDir, inputFileName)
	if _, err := os.Stat(v6Header); err != nil {
		return nil, 0, fmt.Errorf("V6 checkpoint header not found at %s: %w", v6Header, err)
	}
	subtrieChecksums, topTrieChecksum, err := readCheckpointHeader(v6Header, logger)
	if err != nil {
		return nil, 0, fmt.Errorf("could not read V6 checkpoint header: %w", err)
	}
	// The converters address the subtrie part files by index in [0, subtrieCount),
	// so a header declaring a different number of subtries cannot be converted.
	if len(subtrieChecksums) != subtrieCount {
		return nil, 0, fmt.Errorf("V6 checkpoint header declares %v subtrie checksums, expected %v",
			len(subtrieChecksums), subtrieCount)
	}
	if err := allPartFileExist(inputDir, inputFileName, len(subtrieChecksums)); err != nil {
		return nil, 0, fmt.Errorf("V6 part files incomplete for %s/%s: %w", inputDir, inputFileName, err)
	}

	// Validate V7 output is not present (any of the part files).
	v7Existing, err := findCheckpointPartFiles(outputDir, outputFileName)
	if err != nil {
		return nil, 0, fmt.Errorf("could not check existing V7 output files: %w", err)
	}
	if len(v7Existing) != 0 {
		return nil, 0, fmt.Errorf("V7 output already exists: %v", v7Existing)
	}

	return subtrieChecksums, topTrieChecksum, nil
}

// convertCheckpointV6ToV7InMemory converts a V6 checkpoint by loading the entire
// V6 forest into memory, converting it to payloadless tries, and writing them out
// with the V7 writer. Inputs are expected to have been checked by
// [validateV6ToV7Conversion].
//
// No error returns are expected during normal operation.
func convertCheckpointV6ToV7InMemory(
	inputDir string,
	inputFileName string,
	outputDir string,
	outputFileName string,
	logger zerolog.Logger,
	nWorker uint,
) error {
	// Remove any leftover temp part files from a previously interrupted conversion
	// to this output; they are never reused and would otherwise accumulate.
	if err := removeStaleTempFiles(outputDir, outputFileName, logger); err != nil {
		return fmt.Errorf("could not remove stale temp files: %w", err)
	}

	// Read the V6 checkpoint fully — the V6 reader already reads the 16 subtrie
	// part files concurrently. The resulting tries share sub-tries via Go pointer
	// identity, which lets FromV6Tries memoize and avoid redundant conversion.
	v6Header := filePathCheckpointHeader(inputDir, inputFileName)
	v6Tries, err := LoadCheckpoint(v6Header, logger)
	if err != nil {
		return fmt.Errorf("could not load V6 checkpoint: %w", err)
	}

	v7Tries, err := FromV6Tries(v6Tries)
	if err != nil {
		return fmt.Errorf("could not convert V6 tries to payloadless: %w", err)
	}

	// Sanity check: every converted trie must match the source root hash.
	for i, v6 := range v6Tries {
		if v6.RootHash() != v7Tries[i].RootHash() {
			return fmt.Errorf(
				"internal error: converted trie %d root hash mismatch: V6=%s V7=%s",
				i, v6.RootHash(), v7Tries[i].RootHash(),
			)
		}
	}

	logger.Info().
		Int("trie_count", len(v7Tries)).
		Msgf("V6 tries converted, writing V7 checkpoint to %s", path.Join(outputDir, outputFileName))

	if err := StoreCheckpointV7(v7Tries, outputDir, outputFileName, logger, nWorker); err != nil {
		return fmt.Errorf("could not write V7 checkpoint: %w", err)
	}

	return nil
}

// requireV7Filename rejects an output filename that does not carry the V7 suffix.
// This keeps converted files visibly distinct from V6 sources on disk.
func requireV7Filename(fileName string) error {
	if fileName == "" {
		return fmt.Errorf("V7 output filename is empty")
	}
	if len(fileName) <= len(V7FileSuffix) || fileName[len(fileName)-len(V7FileSuffix):] != V7FileSuffix {
		return fmt.Errorf("V7 output filename %q must end with %q", fileName, V7FileSuffix)
	}
	return nil
}
