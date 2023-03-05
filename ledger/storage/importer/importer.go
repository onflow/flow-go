package importer

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/module/util"
)

// ImportLeafNodesFromCheckpoint takes a checkpoint file specified by the dir and fileName,
// reads all the leaf nodes from the checkpoint file, and store them into the given
// storage store.
func ImportLeafNodesFromCheckpoint(dir string, fileName string, logger *zerolog.Logger, store ledger.PayloadStorage) error {
	logger.Info().Msgf("start reading checkpoint file at %v/%v", dir, fileName)

	leafNodes, err := wal.ReadLeafNodesFromCheckpointV6(dir, fileName, logger)
	if err != nil {
		return fmt.Errorf("could not read tries: %w", err)
	}

	logger.Info().Msgf("start importing %v payloads to storage", len(leafNodes))

	batchSize := 1000

	jobs := make(chan []ledger.LeafNode, len(leafNodes)/batchSize+1)

	nBatch := (len(leafNodes) + batchSize - 1) / batchSize

	// creating jobs for batch importing
	logCreatingJobs := util.LogProgress(fmt.Sprintf("creating jobs to store leaf nodes to storage in %v batch", nBatch),
		nBatch, logger)

	for i := 0; i < nBatch; i++ {
		logCreatingJobs(i)

		start := i * batchSize
		end := start + batchSize
		if end > len(leafNodes) {
			end = len(leafNodes)
		}

		leafs := leafNodes[start:end]
		batch := make([]ledger.LeafNode, len(leafs))
		for j, leaf := range leafs {
			batch[j] = *leaf
		}

		jobs <- batch
	}

	nWorker := 10
	results := make(chan error)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create nWorker number of workers to import
	for w := 0; w < nWorker; w++ {
		go func() {
			for batch := range jobs {
				err := store.Add(batch)
				results <- err
				if err != nil {
					cancel()
					return
				}

				select {
				case <-ctx.Done():
					return
				default:
				}
			}
		}()
	}

	logImporting := util.LogProgressWithThreshold(1, "importing leaf nodes to storage", nBatch, logger)
	// waiting for the results
	for i := 0; i < nBatch; i++ {
		logImporting(i)
		err := <-results
		if err != nil {
			return err
		}
	}
	return nil
}

func ValidateFromCheckpoint(dir string, fileName string, logger *zerolog.Logger, store ledger.PayloadStorage) error {

	logger.Info().Msgf("start reading checkpoint file at %v/%v", dir, fileName)

	leafNodes, err := wal.ReadLeafNodesFromCheckpointV6(dir, fileName, logger)
	if err != nil {
		return fmt.Errorf("could not read tries: %w", err)
	}

	logger.Info().Msgf("start validating %v payloads to storage", len(leafNodes))

	jobs := make(chan *ledger.LeafNode, len(leafNodes))

	// creating jobs for batch importing
	logCreatingJobs := util.LogProgress("creating jobs to store leaf nodes to storage", len(leafNodes), logger)

	for i, leafNode := range leafNodes {
		logCreatingJobs(i)
		jobs <- leafNode
	}

	nWorker := 100
	results := make(chan error)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create nWorker number of workers to import
	for w := 0; w < nWorker; w++ {
		go func() {
			for leafNode := range jobs {
				err := checkLeafNodeStored(leafNode, store)
				results <- err
				if err != nil {
					cancel()
					return
				}

				select {
				case <-ctx.Done():
					return
				default:
				}
			}
		}()
	}

	logImporting := util.LogProgressWithThreshold(1, "validated leaf nodes to storage", len(leafNodes), logger)
	// waiting for the results
	for i := 0; i < len(leafNodes); i++ {
		logImporting(i)
		err := <-results
		if err != nil {
			return err
		}
	}

	return nil
}

func checkLeafNodeStored(leafNode *ledger.LeafNode, store ledger.PayloadStorage) error {
	leaf := node.NewLeaf(leafNode.Path, &leafNode.Payload, 0)
	leafHash := leaf.Hash()
	path, payload, err := store.Get(leafHash)
	if err != nil {
		return fmt.Errorf("could not get payload: %w", err)
	}

	if path != leafNode.Path {
		return fmt.Errorf("path does not match (%v != %v)", path, leafNode.Path)
	}

	if !payload.Equals(&leafNode.Payload) {
		return fmt.Errorf("payload does not match")
	}

	return nil
}
