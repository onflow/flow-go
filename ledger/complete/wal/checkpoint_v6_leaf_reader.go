package wal

import (
	"context"
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
)

const (
	// LeafNodeBatchSize is the number of leaf nodes that are pushed to the channel in one
	// message. With many readers and consumers, the channel handoff is much more expensive
	// per leaf node than reading it, so the leaf nodes are pushed in batches.
	LeafNodeBatchSize = 256

	// leafNodeBatchBufferSize is the buffer size, in batches, of the channel the legacy
	// per-leaf-node reader uses to receive the batches it unpacks.
	leafNodeBatchBufferSize = 16
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
//
// This function is a convenience wrapper around
// [OpenAndReadLeafNodesFromCheckpointV6Concurrently] that pushes leaf nodes one by one. Callers
// that want to read a checkpoint with several workers, or that can consume batches of leaf
// nodes, should use [OpenAndReadLeafNodesFromCheckpointV6Concurrently] directly.
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

	leafNodeBatches := make(chan []*LeafNode, leafNodeBatchBufferSize)
	readErrCh := make(chan error, 1)
	go func() {
		readErrCh <- OpenAndReadLeafNodesFromCheckpointV6Concurrently(
			context.Background(), leafNodeBatches, dir, fileName, expectedRootHash, 1, logger)
	}()

	for batch := range leafNodeBatches {
		for _, leafNode := range batch {
			allLeafNodesCh <- leafNode
		}
	}

	return <-readErrCh
}

// OpenAndReadLeafNodesFromCheckpointV6Concurrently takes a channel for pushing the leaf nodes
// that are read from the given checkpoint file specified by dir and fileName, and reads the
// checkpoint's part files with up to workerCount workers in parallel. The leaf nodes are
// pushed in batches of at most [LeafNodeBatchSize] leaf nodes. It returns when the checkpoint
// file has been read and the input channel has been closed.
// It requires the checkpoint file only has one trie.
//
// Each part file holds the nodes of one subtrie and is independent of the other part files,
// so the leaf nodes are pushed to the channel in no particular order. The channel is closed
// once all workers are done.
//
// Expected error returns during normal operation:
//   - [context.Canceled], [context.DeadlineExceeded]: if the context is cancelled
func OpenAndReadLeafNodesFromCheckpointV6Concurrently(
	ctx context.Context,
	leafNodeBatchesCh chan<- []*LeafNode,
	dir string,
	fileName string,
	expectedRootHash ledger.RootHash,
	workerCount int,
	logger zerolog.Logger) (
	errToReturn error) {
	// we are the only senders of the channel, closing it after all workers are done
	defer func() {
		close(leafNodeBatchesCh)
	}()

	if workerCount < 1 {
		return fmt.Errorf("worker count must be at least 1, got %d", workerCount)
	}

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

	// push leaf nodes to leafNodeBatchesCh, reading the part files in parallel; the worker
	// limit is capped by the number of part files, as each part file is read by one worker
	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(workerCount)
	for i, checksum := range subtrieChecksums {
		g.Go(func() error {
			err := readCheckpointSubTrieLeafNodes(gCtx, leafNodeBatchesCh, dir, fileName, i, checksum, logger)
			if err != nil {
				return fmt.Errorf("fail to read checkpoint leaf nodes from %v-th subtrie file: %w", i, err)
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	return nil
}

// readCheckpointSubTrieLeafNodes reads the leaf nodes of the index-th part file of the
// checkpoint and pushes them to the channel in batches.
//
// Expected error returns during normal operation:
//   - [context.Canceled], [context.DeadlineExceeded]: if the context is cancelled
func readCheckpointSubTrieLeafNodes(
	ctx context.Context,
	leafNodeBatchesCh chan<- []*LeafNode,
	dir string,
	fileName string,
	index int,
	checksum uint32,
	logger zerolog.Logger,
) error {
	return processCheckpointSubTrie(dir, fileName, index, checksum, logger,
		func(reader *Crc32Reader, nodesCount uint64) error {
			scratch := make([]byte, 1024*4) // must not be less than 1024

			logging := logProgress(fmt.Sprintf("reading %v-th sub trie roots", index), int(nodesCount), logger)
			dummyChild := &node.Node{}
			batch := make([]*LeafNode, 0, LeafNodeBatchSize)
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
					batch = append(batch, nodeToLeaf(node))
					if len(batch) == cap(batch) {
						if err := sendLeafNodeBatch(ctx, leafNodeBatchesCh, batch); err != nil {
							return err
						}
						// the batch is owned by the consumer from here on
						batch = make([]*LeafNode, 0, LeafNodeBatchSize)
					}
				}

				logging(i)
			}

			return sendLeafNodeBatch(ctx, leafNodeBatchesCh, batch)
		})
}

// sendLeafNodeBatch pushes the given batch of leaf nodes to the channel, unless the context is
// cancelled before the batch can be pushed. It returns the context's error in that case, so
// that senders stop pushing instead of blocking when nobody consumes the channel any more.
// Empty batches are not pushed.
//
// Expected error returns during normal operation:
//   - [context.Canceled], [context.DeadlineExceeded]: if the context is cancelled
func sendLeafNodeBatch(ctx context.Context, leafNodeBatchesCh chan<- []*LeafNode, batch []*LeafNode) error {
	if len(batch) == 0 {
		return nil
	}

	if ctx.Done() == nil {
		// no cancellation possible, e.g. [context.Background]
		leafNodeBatchesCh <- batch
		return nil
	}

	select {
	case leafNodeBatchesCh <- batch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
