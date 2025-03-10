package pruner

import (
	"github.com/cockroachdb/pebble"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// NewChunkDataPackPruningEngine creates a component that prunes chunk data packs
// from root to the latest sealed block.
func NewChunkDataPackPruningEngine(
	log zerolog.Logger,
	metrics module.ExecutionMetrics,
	state protocol.State,
	protocolDB storage.DB,
	headers storage.Headers,
	chunkDataPacks storage.ChunkDataPacks,
	results storage.ExecutionResults,
	chunkDataPacksDB *pebble.DB,
	config PruningConfig,
) *component.ComponentManager {
	return component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()

			err := LoopPruneExecutionDataFromRootToLatestSealed(
				ctx, log.With().Str("component", "CDP-pruner").Logger(), metrics,
				state, protocolDB, headers, chunkDataPacks, results, chunkDataPacksDB, config)
			if err != nil {
				ctx.Throw(err)
			}
		}).
		Build()
}
