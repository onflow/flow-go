package pruner

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

func NewChunkDataPackPruningEngine(
	log zerolog.Logger,
	state protocol.State,
	badgerDB *badger.DB,
	headers storage.Headers,
	chunkDataPacks storage.ChunkDataPacks,
	results storage.ExecutionResults,
	db storage.DB,
	config PruningConfig,
	callback func(),
) *component.ComponentManager {
	return component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()

			err := LoopPruneExecutionDataFromRootToLatestSealed(
				ctx, state, badgerDB, headers, chunkDataPacks, results, db, config, callback)
			if err != nil {
				ctx.Throw(err)
			}
		}).
		Build()
}
