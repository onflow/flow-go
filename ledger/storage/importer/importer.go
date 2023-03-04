package importer

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
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

	// creating jobs for batch importing
	logCreatingJobs := util.LogProgress("creating jobs to store leaf nodes to storage", len(leafNodes), logger)
	batchSize := 1000

	jobs := make(chan []ledger.LeafNode, len(leafNodes)/batchSize+1)

	batch := make([]ledger.LeafNode, 0, batchSize)
	nBatch := 0
	for i := 0; i < len(leafNodes); i += batchSize {
		logCreatingJobs(i)
		end := i + batchSize
		if end > len(leafNodes) {
			end = len(leafNodes)
		}

		thisBatchSize := end - i
		for j := 0; j < thisBatchSize; j++ {
			batch[j] = *leafNodes[i+j]
		}

		err := store.Add(batch[:thisBatchSize])
		if err != nil {
			return fmt.Errorf("could not store leaf nodes: %w", err)
		}
		nBatch++
	}

	if len(batch) > 0 {
		jobs <- batch
		nBatch++
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
