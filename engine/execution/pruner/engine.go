package pruner

import (
	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// NewChunkDataPackPruningEngine creates a component that prunes chunk data packs
// from root to the latest sealed block.
func NewChunkDataPackPruningEngine(
	log zerolog.Logger,
	state protocol.State,
	badgerDB *badger.DB,
	headers storage.Headers,
	chunkDataPacks storage.ChunkDataPacks,
	results storage.ExecutionResults,
	chunkDataPacksDB *pebble.DB,
	config PruningConfig,
) *component.ComponentManager {
	return component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()

			callback := func() {
				log.Info().Msgf("Pruning iteration finished")
			}

			err := LoopPruneExecutionDataFromRootToLatestSealed(
				ctx, state, badgerDB, headers, chunkDataPacks, results, chunkDataPacksDB, config, callback)
			if err != nil {
				ctx.Throw(err)
			}
		}).
		Build()
}
