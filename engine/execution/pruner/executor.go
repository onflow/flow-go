package pruner

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/block_iterator/executor"
	"github.com/onflow/flow-go/storage"
)

type ExecutionDBPruner struct {
	*executor.AggregatedExecutor
}

var _ executor.IterationExecutor = (*ExecutionDBPruner)(nil)

func NewExecutionDataPruner() *ExecutionDBPruner {
	chunkDataPackPruner := &ChunkDataPackPruner{} // TODO add depenedencies
	return &ExecutionDBPruner{
		executor.NewAggregatedExecutor([]executor.IterationExecutor{
			chunkDataPackPruner,
		}),
	}
}

type ChunkDataPackPruner struct {
	chunkDataPacks storage.ChunkDataPacks
	results        storage.ExecutionResults
}

var _ executor.IterationExecutor = (*ChunkDataPackPruner)(nil)

func (c *ChunkDataPackPruner) ExecuteByBlockID(blockID flow.Identifier, batch storage.ReaderBatchWriter) (exception error) {
	return nil
}
