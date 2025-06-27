package persisters

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_syncing/persisters/stores"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// BlockPersister handles persisting of execution data for all PersisterStore-s to database.
// Each BlockPersister instance is created for ONE specific block
type BlockPersister struct {
	log zerolog.Logger

	persisterStores []stores.PersisterStore
	protocolDB      storage.DB
	executionResult *flow.ExecutionResult
	header          *flow.Header
}

// NewBlockPersister creates a new block persister.
func NewBlockPersister(
	log zerolog.Logger,
	protocolDB storage.DB,
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

	err := p.protocolDB.WithReaderBatchWriter(func(batch storage.ReaderBatchWriter) error {
		for _, persister := range p.persisterStores {
			if err := persister.Persist(batch); err != nil {
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
