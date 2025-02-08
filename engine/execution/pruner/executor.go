package pruner

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/block_iterator/executor"
	"github.com/onflow/flow-go/module/pruner/pruners"
	"github.com/onflow/flow-go/storage"
)

type ChunkDataPackPruner struct {
	*pruners.ChunkDataPackPruner
}

var _ executor.IterationExecutor = (*ChunkDataPackPruner)(nil)

func NewChunkDataPackPruner(chunkDataPacks storage.ChunkDataPacks, results storage.ExecutionResults) *ChunkDataPackPruner {
	return &ChunkDataPackPruner{
		ChunkDataPackPruner: pruners.NewChunkDataPackPruner(chunkDataPacks, results),
	}
}

func (c *ChunkDataPackPruner) ExecuteByBlockID(blockID flow.Identifier, batch storage.ReaderBatchWriter) (exception error) {
	return c.PruneByBlockID(blockID, batch)
}
