package wal

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
)

type LeafNode struct {
	Hash    hash.Hash
	Path    ledger.Path
	Payload *ledger.Payload
}

func nodeToLeaf(leaf *node.Node) *LeafNode {
	return &LeafNode{
		Hash:    leaf.Hash(),
		Path:    *leaf.Path(),
		Payload: leaf.Payload(),
	}
}

// OpenAndReadLeafNodesFromCheckpointV6 takes a channel for pushing the leaf nodes that are read from
// the given checkpoint file specified by dir and fileName.
// It returns when finish reading the checkpoint file and the input channel can be closed.
// It requires the checkpoint file only has one trie.
func OpenAndReadLeafNodesFromCheckpointV6(
	allLeafNodesCh chan<- *LeafNode,
	dir string,
	fileName string,
	expectedRootHash ledger.RootHash,
	logger zerolog.Logger) (
	errToReturn error) {
	// we are the only sender of the channel, closing it after done
	defer func() {
		close(allLeafNodesCh)
	}()

	err := checkpointHasSingleRootHash(logger, dir, fileName, expectedRootHash)
	if err != nil {
		return fmt.Errorf("fail to check checkpoint has single root hash: %w", err)
	}

	filepath := filePathCheckpointHeader(dir, fileName)

	f, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("could not open file %v: %w", filepath, err)
	}
	defer func(file *os.File) {
		errToReturn = closeAndMergeError(file, errToReturn)
	}(f)

	subtrieChecksums, _, err := readCheckpointHeader(filepath, logger)
	if err != nil {
		return fmt.Errorf("could not read header: %w", err)
	}

	// ensure all checkpoint part file exists, might return os.ErrNotExist error
	// if a file is missing
	err = allPartFileExist(dir, fileName, len(subtrieChecksums))
	if err != nil {
		return fmt.Errorf("fail to check all checkpoint part file exist: %w", err)
	}

	// push leaf nodes to allLeafNodesCh
	for i, checksum := range subtrieChecksums {
		err := readCheckpointSubTrieLeafNodes(allLeafNodesCh, dir, fileName, i, checksum, logger)
		if err != nil {
			return fmt.Errorf("fail to read checkpoint leaf nodes from %v-th subtrie file: %w", i, err)
		}
	}

	return nil
}

func readCheckpointSubTrieLeafNodes(leafNodesCh chan<- *LeafNode, dir string, fileName string, index int, checksum uint32, logger zerolog.Logger) error {
	return processCheckpointSubTrie(dir, fileName, index, checksum, logger,
		func(reader *Crc32Reader, nodesCount uint64) error {
			scratch := make([]byte, 1024*4) // must not be less than 1024

			logging := logProgress(fmt.Sprintf("reading %v-th sub trie roots", index), int(nodesCount), logger)
			dummyChild := &node.Node{}
			for i := uint64(1); i <= nodesCount; i++ {
				node, err := flattener.ReadNode(reader, scratch, func(nodeIndex uint64) (*node.Node, error) {
					if nodeIndex >= i {
						return nil, fmt.Errorf("sequence of serialized nodes does not satisfy Descendents-First-Relationship")
					}
					return dummyChild, nil
				})
				if err != nil {
					return fmt.Errorf("cannot read node %d: %w", i, err)
				}
				if node.IsLeaf() {
					leafNodesCh <- nodeToLeaf(node)
				}

				logging(i)
			}
			return nil
		})
}

// CheckpointStats holds statistics about a checkpoint's nodes.
type CheckpointStats struct {
	RootHash         ledger.RootHash
	InterimNodeCount uint64
	LeafNodeCount    uint64
	TotalPayloadSize uint64
}

// ReadCheckpointStats reads checkpoint statistics without loading the full trie into memory.
// It iterates through all nodes in the checkpoint files and counts interim nodes, leaf nodes,
// and total payload size.
// This function requires the checkpoint to contain exactly one trie.
//
// Expected errors during normal operation:
//   - various I/O errors if checkpoint files are missing or corrupted
func ReadCheckpointStats(dir string, fileName string, logger zerolog.Logger) (CheckpointStats, error) {
	// Verify checkpoint has exactly one trie
	roots, err := ReadTriesRootHash(logger, dir, fileName)
	if err != nil {
		return CheckpointStats{}, fmt.Errorf("could not read checkpoint root hashes: %w", err)
	}
	if len(roots) != 1 {
		return CheckpointStats{}, fmt.Errorf("checkpoint must contain exactly 1 trie, but has %d", len(roots))
	}

	filepath := filePathCheckpointHeader(dir, fileName)
	subtrieChecksums, topTrieChecksum, err := readCheckpointHeader(filepath, logger)
	if err != nil {
		return CheckpointStats{}, fmt.Errorf("could not read header: %w", err)
	}

	// Ensure all checkpoint part files exist
	err = allPartFileExist(dir, fileName, len(subtrieChecksums))
	if err != nil {
		return CheckpointStats{}, fmt.Errorf("checkpoint part file missing: %w", err)
	}

	var stats CheckpointStats
	stats.RootHash = roots[0]

	// Process subtrie files (0-15)
	for i, checksum := range subtrieChecksums {
		interim, leaf, payloadSize, err := readCheckpointSubTrieStats(dir, fileName, i, checksum, logger)
		if err != nil {
			return CheckpointStats{}, fmt.Errorf("failed to read stats from %d-th subtrie file: %w", i, err)
		}
		stats.InterimNodeCount += interim
		stats.LeafNodeCount += leaf
		stats.TotalPayloadSize += payloadSize
	}

	// Process top trie file (file 016)
	interim, leaf, payloadSize, err := readCheckpointTopTrieStats(dir, fileName, topTrieChecksum, logger)
	if err != nil {
		return CheckpointStats{}, fmt.Errorf("failed to read stats from top trie file: %w", err)
	}
	stats.InterimNodeCount += interim
	stats.LeafNodeCount += leaf
	stats.TotalPayloadSize += payloadSize

	return stats, nil
}

func readCheckpointSubTrieStats(dir string, fileName string, index int, checksum uint32, logger zerolog.Logger) (
	interimCount uint64, leafCount uint64, payloadSize uint64, err error) {
	err = processCheckpointSubTrie(dir, fileName, index, checksum, logger,
		func(reader *Crc32Reader, nodesCount uint64) error {
			scratch := make([]byte, 1024*4) // must not be less than 1024
			dummyChild := &node.Node{}

			for i := uint64(1); i <= nodesCount; i++ {
				n, err := flattener.ReadNode(reader, scratch, func(nodeIndex uint64) (*node.Node, error) {
					if nodeIndex >= i {
						return nil, fmt.Errorf("sequence of serialized nodes does not satisfy Descendents-First-Relationship")
					}
					return dummyChild, nil
				})
				if err != nil {
					return fmt.Errorf("cannot read node %d: %w", i, err)
				}
				if n.IsLeaf() {
					leafCount++
					if n.Payload() != nil {
						payloadSize += uint64(n.Payload().Size())
					}
				} else {
					interimCount++
				}
			}
			return nil
		})
	return
}

func readCheckpointTopTrieStats(dir string, fileName string, expectedChecksum uint32, logger zerolog.Logger) (
	interimCount uint64, leafCount uint64, payloadSize uint64, errToReturn error) {

	filepath, _ := filePathTopTries(dir, fileName)
	errToReturn = withFile(logger, filepath, func(file *os.File) error {
		// Validate header
		err := validateFileHeader(MagicBytesCheckpointToptrie, VersionV6, file)
		if err != nil {
			return err
		}

		// Read footer to get node count
		topLevelNodesCount, _, checksum, err := readTopTriesFooter(file)
		if err != nil {
			return fmt.Errorf("could not read top tries footer: %w", err)
		}

		if expectedChecksum != checksum {
			return fmt.Errorf("checksum mismatch: header has %v, top trie file has %v", expectedChecksum, checksum)
		}

		// Seek back to start for reading nodes
		_, err = file.Seek(0, io.SeekStart)
		if err != nil {
			return fmt.Errorf("could not seek to start: %w", err)
		}

		reader := NewCRC32Reader(bufio.NewReaderSize(file, defaultBufioReadSize))

		// Skip header (already validated)
		_, _, err = readFileHeader(reader)
		if err != nil {
			return fmt.Errorf("could not read header: %w", err)
		}

		// Skip subtrie node count field (8 bytes)
		buf := make([]byte, encNodeCountSize)
		_, err = io.ReadFull(reader, buf)
		if err != nil {
			return fmt.Errorf("could not read subtrie node count: %w", err)
		}

		// Read top level nodes
		scratch := make([]byte, 1024*4)
		dummyChild := &node.Node{}

		for i := uint64(1); i <= topLevelNodesCount; i++ {
			n, err := flattener.ReadNode(reader, scratch, func(nodeIndex uint64) (*node.Node, error) {
				return dummyChild, nil
			})
			if err != nil {
				return fmt.Errorf("cannot read top level node %d: %w", i, err)
			}
			if n.IsLeaf() {
				leafCount++
				if n.Payload() != nil {
					payloadSize += uint64(n.Payload().Size())
				}
			} else {
				interimCount++
			}
		}

		return nil
	})
	return
}
