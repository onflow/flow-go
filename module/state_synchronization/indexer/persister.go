package indexer

import (
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/store/inmemory/unsynchronized"
	"github.com/onflow/flow-go/utils/logging"
)

// Persister handles transferring data from in-memory storage to permanent storage.
type Persister struct {
	log                  zerolog.Logger
	inMemoryRegisters    *unsynchronized.Registers
	inMemoryEvents       *unsynchronized.Events
	inMemoryCollections  *unsynchronized.Collections
	inMemoryTransactions *unsynchronized.Transactions
	inMemoryResults      *unsynchronized.LightTransactionResults
	registers            storage.RegisterIndex
	events               storage.Events
	collections          storage.Collections
	transactions         storage.Transactions
	results              storage.LightTransactionResults
	executionResult      *flow.ExecutionResult
	header               *flow.Header

	LastPersistedSealedExecutionResult *flow.ExecutionResult
}

// NewPersister creates a new persister for transferring data from in-memory storage to permanent storage.
func NewPersister(
	log zerolog.Logger,
	inMemoryRegisters *unsynchronized.Registers,
	inMemoryEvents *unsynchronized.Events,
	inMemoryCollections *unsynchronized.Collections,
	inMemoryTransactions *unsynchronized.Transactions,
	inMemoryResults *unsynchronized.LightTransactionResults,
	registers storage.RegisterIndex,
	events storage.Events,
	collections storage.Collections,
	transactions storage.Transactions,
	results storage.LightTransactionResults,
	executionResult *flow.ExecutionResult,
	header *flow.Header,
) *Persister {
	persister := &Persister{
		log:                  log.With().Str("component", "persister").Logger(),
		inMemoryRegisters:    inMemoryRegisters,
		inMemoryEvents:       inMemoryEvents,
		inMemoryCollections:  inMemoryCollections,
		inMemoryTransactions: inMemoryTransactions,
		inMemoryResults:      inMemoryResults,
		registers:            registers,
		events:               events,
		collections:          collections,
		transactions:         transactions,
		results:              results,
		executionResult:      executionResult,
		header:               header,
	}

	persister.log.Info().
		Uint64("height", header.Height).
		Msg("persister initialized")

	return persister
}

// AddToBatch adds data from in-memory storage to the provided batch.
// It processes events, transaction results, registers, collections, and transactions
// from their respective in-memory storages to permanent storage.
// Note: This method doesn't commit the batch - that should be done by the caller.
func (p *Persister) AddToBatch(batch storage.Batch) error {
	log := p.log.With().
		Hex("block_id", logging.ID(p.executionResult.BlockID)).
		Uint64("height", p.header.Height).
		Logger()

	log.Debug().Msg("adding execution data to batch")

	start := time.Now()

	if err := p.addEventsToBatch(batch); err != nil {
		return err
	}

	if err := p.addResultsToBatch(batch); err != nil {
		return err
	}

	if err := p.persistRegisters(); err != nil {
		return err
	}

	if err := p.persistCollections(); err != nil {
		return err
	}

	if err := p.persistTransactions(); err != nil {
		return err
	}

	p.LastPersistedSealedExecutionResult = p.executionResult

	duration := time.Since(start)
	log.Debug().
		Dur("duration_ms", duration).
		Msg("successfully prepared execution data for persistence")

	return nil
}

// addEventsToBatch adds events from in-memory storage to the batch.
// If there are no events, this is a no-op.
func (p *Persister) addEventsToBatch(batch storage.Batch) error {
	if eventsList := p.inMemoryEvents.Data(); len(eventsList) > 0 {
		if err := p.events.BatchStore(p.executionResult.BlockID, []flow.EventsList{eventsList}, batch); err != nil {
			return fmt.Errorf("could not add events to batch: %w", err)
		}
		p.log.Debug().Int("event_count", len(eventsList)).Msg("added events to batch")
	}

	return nil
}

// addResultsToBatch adds transaction results from in-memory storage to the batch.
// If there are no results, this is a no-op.
func (p *Persister) addResultsToBatch(batch storage.Batch) error {
	if results := p.inMemoryResults.Data(); len(results) > 0 {
		if err := p.results.BatchStore(p.executionResult.BlockID, results, batch); err != nil {
			return fmt.Errorf("could not add transaction results to batch: %w", err)
		}
		p.log.Debug().Int("result_count", len(results)).Msg("added transaction results to batch")
	}

	return nil
}

// persistRegisters persists registers from in-memory to permanent storage.
// If there are no registers, this is a no-op.
func (p *Persister) persistRegisters() error {
	if registerData := p.inMemoryRegisters.Data(); len(registerData) > 0 {
		start := time.Now()

		if err := p.registers.Store(registerData, p.header.Height); err != nil {
			return fmt.Errorf("could not persist registers: %w", err)
		}

		p.log.Debug().
			Int("register_count", len(registerData)).
			Dur("duration_ms", time.Since(start)).
			Msg("persisted registers")
	}

	return nil
}

// persistCollections persists collections from in-memory to permanent storage.
// It skips collections that already exist in permanent storage.
// If there are no collections, this is a no-op.
func (p *Persister) persistCollections() error {
	collections := p.inMemoryCollections.Collections()

	if len(collections) == 0 {
		return nil
	}

	start := time.Now()
	persistedCount := 0

	for _, collection := range collections {
		light := collection.Light()
		if err := p.collections.StoreLightAndIndexByTransaction(&light); err != nil {
			if errors.Is(err, storage.ErrAlreadyExists) {
				// Skip if already exists
				p.log.Debug().
					Hex("collection_id", logging.Entity(light)).
					Msg("collection already exists in permanent storage")
				continue
			}
			return fmt.Errorf("could not persist collection: %w", err)
		}
		persistedCount++
	}

	if persistedCount > 0 {
		p.log.Debug().
			Int("persisted_collections", persistedCount).
			Int("total_collections", len(collections)).
			Dur("duration_ms", time.Since(start)).
			Msg("persisted collections")
	}

	return nil
}

// persistTransactions persists transactions from in-memory to permanent storage.
// It skips transactions that already exist in permanent storage.
// If there are no transactions, this is a no-op.
func (p *Persister) persistTransactions() error {
	transactions := p.inMemoryTransactions.Data()

	if len(transactions) == 0 {
		return nil
	}

	start := time.Now()
	persistedCount := 0

	for _, tx := range transactions {
		if err := p.transactions.Store(&tx); err != nil {
			if errors.Is(err, storage.ErrAlreadyExists) {
				// Skip if already exists
				continue
			}
			return fmt.Errorf("could not persist transaction (%s): %w", tx.ID().String(), err)
		}
		persistedCount++
	}

	if persistedCount > 0 {
		p.log.Debug().
			Int("persisted_transactions", persistedCount).
			Int("total_transactions", len(transactions)).
			Dur("duration_ms", time.Since(start)).
			Msg("persisted transactions")
	}

	return nil
}
