package persisters

import (
	"fmt"
	"time"

	"github.com/jordanschalm/lockctx"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync/persisters/stores"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// BlockPersister stores execution data for a single execution result into the database.
// It uses the set of [stores.PersisterStore] to persist the data with a single atomic batch operation.
// To ensure the database only contains data certified by the protocol, the block persister must
// only be called for sealed execution results.
type BlockPersister struct {
	log zerolog.Logger

	persisterStores []stores.PersisterStore
	protocolDB      storage.DB
	lockManager     lockctx.Manager
	executionResult *flow.ExecutionResult
}

// NewBlockPersister creates a new block persister.
func NewBlockPersister(
	log zerolog.Logger,
	protocolDB storage.DB,
	lockManager lockctx.Manager,
	executionResult *flow.ExecutionResult,
	persisterStores []stores.PersisterStore,
) *BlockPersister {
	log = log.With().
		Str("component", "block_persister").
		Hex("execution_result_id", logging.ID(executionResult.ID())).
		Hex("block_id", logging.ID(executionResult.BlockID)).
		Logger()

	log.Info().
		Int("batch_persisters_count", len(persisterStores)).
		Msg("block persisters initialized")

	return &BlockPersister{
		log:             log,
		persisterStores: persisterStores,
		protocolDB:      protocolDB,
		executionResult: executionResult,
		lockManager:     lockManager,
	}
}

// Persist atomically stores all data into the database using the configured persister stores.
//
// No error returns are expected during normal operations
func (p *BlockPersister) Persist() error {
	p.log.Debug().Msg("started to persist execution data")
	start := time.Now()

	err := storage.WithLocks(p.lockManager, []string{
		storage.LockInsertCollection,
		storage.LockInsertLightTransactionResult,
	}, func(lctx lockctx.Context) error {
		return p.protocolDB.WithReaderBatchWriter(func(batch storage.ReaderBatchWriter) error {
			for _, persister := range p.persisterStores {
				if err := persister.Persist(lctx, batch); err != nil {
					return err
				}
			}
			return nil
		})
	})

	if err != nil {
		return fmt.Errorf("failed to commit batch: %w", err)
	}

	p.log.Debug().
		Dur("duration_ms", time.Since(start)).
		Msg("successfully prepared execution data for persistence")

	return nil
}
