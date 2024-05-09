package wal

import (
	"fmt"
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
func OpenAndReadLeafNodesFromCheckpointV6(allLeafNodesCh chan<- *LeafNode, dir string, fileName string, logger zerolog.Logger) (errToReturn error) {
	// we are the only sender of the channel, closing it after done
	defer func() {
		close(allLeafNodesCh)
	}()

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
