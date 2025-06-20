package pipeline

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// BlockPersister handles transferring data from in-memory storages to permanent storages
// for a single block.
//
// - Each BlockPersister instance is created for ONE specific block
// - All `inMemory*` storages contain data ONLY for this specific block, they are not shared
type BlockPersister struct {
	log zerolog.Logger

	persisterStores []PersisterStore
	protocolDB      storage.DB
	executionResult *flow.ExecutionResult
	header          *flow.Header
}

// NewBlockPersister creates a new persister with dependency injection.
func NewBlockPersister(
	log zerolog.Logger,
	persisterStores []PersisterStore,
	protocolDB storage.DB,
	executionResult *flow.ExecutionResult,
	header *flow.Header,
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
		Uint64("height", header.Height).
		Int("batch_persisters_count", len(persisterStores)).
		Msg("block persister initialized")

	return persister
}

// Persist save data from in-memory storages to the provided persisted storages and commit updates to the database.
// No errors are expected during normal operations
func (p *BlockPersister) Persist() error {
	p.log.Debug().Msg("adding execution data to batch")
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

	// TODO: include update to latestPersistedSealedResultBlockHeight in the batch

	duration := time.Since(start)

	p.log.Debug().
		Dur("duration_ms", duration).
		Msg("successfully prepared execution data for persistence")

	return nil
}
