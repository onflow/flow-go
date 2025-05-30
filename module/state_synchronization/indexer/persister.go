package indexer

import (
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

	if err := p.persistCollections(batch); err != nil {
		return err
	}

	if err := p.persistTransactions(batch); err != nil {
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
// Registers must be stored for every height, even if it's an empty set.
func (p *Persister) persistRegisters() error {
	registerData := p.inMemoryRegisters.Data()

	// Always store registers for every height to maintain height continuity
	if err := p.registers.Store(registerData, p.header.Height); err != nil {
		return fmt.Errorf("could not persist registers: %w", err)
	}

	p.log.Debug().
		Int("register_count", len(registerData)).
		Msg("persisted registers")

	return nil
}

// persistCollections persists collections from in-memory to permanent storage.
// It skips collections that already exist in permanent storage.
// If there are no collections, this is a no-op.
func (p *Persister) persistCollections(batch storage.Batch) error {
	if collections := p.inMemoryCollections.LightCollections(); len(collections) > 0 {
		if err := p.collections.BatchStoreLightAndIndexByTransaction(collections, batch); err != nil {
			return fmt.Errorf("could not add collections to batch: %w", err)
		}
		p.log.Debug().Int("collections_count", len(collections)).Msg("added collections to batch")
	}

	return nil
}

// persistTransactions persists transactions from in-memory to permanent storage.
// It skips transactions that already exist in permanent storage.
// If there are no transactions, this is a no-op.
func (p *Persister) persistTransactions(batch storage.Batch) error {
	if transactions := p.inMemoryTransactions.Data(); len(transactions) > 0 {
		if err := p.transactions.BatchStore(transactions, batch); err != nil {
			return fmt.Errorf("could not add transactions to batch: %w", err)
		}
		p.log.Debug().Int("transactions_count", len(transactions)).Msg("added transactions to batch")
	}

	return nil
}
