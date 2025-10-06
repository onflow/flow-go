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

// BlockPersister handles persisting of execution data for all PersisterStore-s to database.
// Each BlockPersister instance is created for ONE specific block
type BlockPersister struct {
	log zerolog.Logger

	persisterStores []stores.PersisterStore
	protocolDB      storage.DB
	lockManager     lockctx.Manager
	executionResult *flow.ExecutionResult
	header          *flow.Header
}

// NewBlockPersister creates a new block persister.
func NewBlockPersister(
	log zerolog.Logger,
	protocolDB storage.DB,
	lockManager lockctx.Manager,
	executionResult *flow.ExecutionResult,
	header *flow.Header,
	persisterStores []stores.PersisterStore,
) *BlockPersister {
	log = log.With().
		Str("component", "block_persister").
		Hex("block_id", logging.ID(executionResult.BlockID)).
		Uint64("height", header.Height).
		Logger()

	persister := &BlockPersister{
		log:             log,
		persisterStores: persisterStores,
		protocolDB:      protocolDB,
		executionResult: executionResult,
		header:          header,
		lockManager:     lockManager,
	}

	persister.log.Info().
		Int("batch_persisters_count", len(persisterStores)).
		Msg("block persisters initialized")

	return persister
}

// Persist save data in provided persisted stores and commit updates to the database.
// No errors are expected during normal operations
func (p *BlockPersister) Persist() error {
	p.log.Debug().Msg("started to persist execution data")
	start := time.Now()

	lctx := p.lockManager.NewContext()
	defer lctx.Release()
	err := lctx.AcquireLock(storage.LockInsertCollection)
	if err != nil {
		return fmt.Errorf("could not acquire lock for inserting light collections: %w", err)
	}

	err = lctx.AcquireLock(storage.LockInsertEvent)
	if err != nil {
		return fmt.Errorf("could not acquire lock for inserting events: %w", err)
	}

	err = lctx.AcquireLock(storage.LockInsertLightTransactionResult)
	if err != nil {
		return fmt.Errorf("could not acquire lock for inserting light transaction results: %w", err)
	}

	err = p.protocolDB.WithReaderBatchWriter(func(batch storage.ReaderBatchWriter) error {
		for _, persister := range p.persisterStores {
			if err := persister.Persist(lctx, batch); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to commit batch: %w", err)
	}

	p.log.Debug().
		Dur("duration_ms", time.Since(start)).
		Msg("successfully prepared execution data for persistence")

	return nil
}
