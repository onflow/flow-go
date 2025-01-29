package pruner

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/block_iterator/executor"
	"github.com/onflow/flow-go/storage"
)

type ChunkDataPackPruner struct {
	chunkDataPacks storage.ChunkDataPacks
	results        storage.ExecutionResults
}

var _ executor.IterationExecutor = (*ChunkDataPackPruner)(nil)

// TODO: replace when https://github.com/onflow/flow-go/pull/6919 is merged
func NewChunKDataPackPruner(chunkDataPacks storage.ChunkDataPacks, results storage.ExecutionResults) *ChunkDataPackPruner {
	return &ChunkDataPackPruner{
		chunkDataPacks: chunkDataPacks,
		results:        results,
	}
}

func (c *ChunkDataPackPruner) ExecuteByBlockID(blockID flow.Identifier, batch storage.ReaderBatchWriter) (exception error) {
	return nil
}
