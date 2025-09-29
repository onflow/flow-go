package pruners

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type ChunkDataPackPruner struct {
	chunkDataPacks storage.ChunkDataPacks
	results        storage.ExecutionResults
}

func NewChunkDataPackPruner(chunkDataPacks storage.ChunkDataPacks, results storage.ExecutionResults) *ChunkDataPackPruner {
	return &ChunkDataPackPruner{
		chunkDataPacks: chunkDataPacks,
		results:        results,
	}
}

func (p *ChunkDataPackPruner) PruneByBlockID(blockID flow.Identifier, batchWriter storage.ReaderBatchWriter) error {
	result, err := p.results.ByBlockID(blockID)

	// result not found, then chunk data pack must not exist either
	if errors.Is(err, storage.ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to get execution result by block ID: %w", err)
	}

	// collect all chunk IDs to remove
	chunkIDs := make([]flow.Identifier, 0, len(result.Chunks))
	for _, chunk := range result.Chunks {
		chunkIDs = append(chunkIDs, chunk.ID())
	}

	// remove all chunk data packs in a single batch operation
	if len(chunkIDs) > 0 {
		err := p.chunkDataPacks.BatchRemove(chunkIDs, batchWriter)
		if err != nil {
			return fmt.Errorf("could not remove chunk data packs for block id %v: %w", blockID, err)
		}
	}

	return nil
}
