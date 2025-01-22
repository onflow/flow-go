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

	for _, chunk := range result.Chunks {
		chunkID := chunk.ID()
		// remove chunk data pack
		err := p.chunkDataPacks.BatchDelete(chunkID, batchWriter)
		if errors.Is(err, storage.ErrNotFound) {
			continue
		}

		if err != nil {
			return fmt.Errorf("could not remove chunk id %v for block id %v: %w", chunkID, blockID, err)
		}
	}

	return nil
}
