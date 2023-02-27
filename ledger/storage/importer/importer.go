package importer

import (
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
	leafNodes, err := wal.ReadLeafNodesFromCheckpointV6(dir, fileName, logger)
	if err != nil {
		return fmt.Errorf("could not read tries: %w", err)
	}

	logProgress := util.LogProgress("importing leaf nodes to storage", len(leafNodes), logger)
	batchSize := 10
	batch := make([]ledger.LeafNode, 0, batchSize)

	for i := 0; i < len(leafNodes); i += batchSize {
		logProgress(i)
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
	}

	if len(batch) > 0 {
		err := store.Add(batch)
		if err != nil {
			return fmt.Errorf("could not store leaf nodes: %w", err)
		}
	}

	return nil
}
