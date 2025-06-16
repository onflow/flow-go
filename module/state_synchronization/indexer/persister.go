package indexer

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"go.uber.org/multierr"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/store/inmemory/unsynchronized"
	"github.com/onflow/flow-go/utils/logging"
)

// Persister handles transferring data from in-memory storages to permanent storages.
type Persister struct {
	log zerolog.Logger

	inMemoryRegisters      *unsynchronized.Registers
	inMemoryEvents         *unsynchronized.Events
	inMemoryCollections    *unsynchronized.Collections
	inMemoryTransactions   *unsynchronized.Transactions
	inMemoryResults        *unsynchronized.LightTransactionResults
	inMemoryTxResultErrMsg *unsynchronized.TransactionResultErrorMessages

	registers      storage.RegisterIndex
	events         storage.Events
	collections    storage.Collections
	transactions   storage.Transactions
	results        storage.LightTransactionResults
	txResultErrMsg storage.TransactionResultErrorMessages
	protocolDB     storage.DB

	executionResult *flow.ExecutionResult
	header          *flow.Header
}

// NewPersister creates a new persister.
func NewPersister(
	log zerolog.Logger,
	inMemoryRegisters *unsynchronized.Registers,
	inMemoryEvents *unsynchronized.Events,
	inMemoryCollections *unsynchronized.Collections,
	inMemoryTransactions *unsynchronized.Transactions,
	inMemoryResults *unsynchronized.LightTransactionResults,
	inMemoryTxResultErrMsg *unsynchronized.TransactionResultErrorMessages,
	registers storage.RegisterIndex,
	events storage.Events,
	collections storage.Collections,
	transactions storage.Transactions,
	results storage.LightTransactionResults,
	txResultErrMsg storage.TransactionResultErrorMessages,
	protocolDB storage.DB,
	executionResult *flow.ExecutionResult,
	header *flow.Header,
) *Persister {
	log = log.With().
		Str("component", "persister").
		Hex("block_id", logging.ID(executionResult.BlockID)).
		Uint64("height", header.Height).
		Logger()

	persister := &Persister{
		log:                    log,
		inMemoryRegisters:      inMemoryRegisters,
		inMemoryEvents:         inMemoryEvents,
		inMemoryCollections:    inMemoryCollections,
		inMemoryTransactions:   inMemoryTransactions,
		inMemoryResults:        inMemoryResults,
		inMemoryTxResultErrMsg: inMemoryTxResultErrMsg,
		registers:              registers,
		events:                 events,
		collections:            collections,
		transactions:           transactions,
		results:                results,
		txResultErrMsg:         txResultErrMsg,
		executionResult:        executionResult,
		header:                 header,
		protocolDB:             protocolDB,
	}

	persister.log.Info().
		Uint64("height", header.Height).
		Msg("persister initialized")

	return persister
}

// Persist save data from in-memory storages to the provided persisted storages and commit updates to the database.
// No errors are expected during normal operations
func (p *Persister) Persist() error {
	// Create a batch for atomic updates
	batch := p.protocolDB.NewBatch()

	var err error
	defer func() {
		err = multierr.Combine(err, batch.Close())
	}()

	p.log.Debug().Msg("adding execution data to batch")

	start := time.Now()

	if err = p.persistRegisters(); err != nil {
		return err
	}

	if err = p.addEventsToBatch(batch); err != nil {
		return err
	}

	if err = p.addResultsToBatch(batch); err != nil {
		return err
	}

	if err = p.addCollectionsToBatch(batch); err != nil {
		return err
	}

	if err = p.addTransactionsToBatch(batch); err != nil {
		return err
	}

	if err = p.addTransactionResultErrorMessagesToBatch(batch); err != nil {
		return err
	}

	// TODO: include update to latestPersistedSealedResultBlockHeight in the batch

	if err = batch.Commit(); err != nil {
		return fmt.Errorf("failed to commit batch: %w", err)
	}

	duration := time.Since(start)

	p.log.Debug().
		Dur("duration_ms", duration).
		Int("events_count", len(p.inMemoryEvents.Data())).
		Int("collection_count", len(p.inMemoryCollections.Data())).
		Int("transactions_count", len(p.inMemoryTransactions.Data())).
		Int("transaction_results_count", len(p.inMemoryResults.Data())).
		Int("transaction_result_error_messages_count", len(p.inMemoryTxResultErrMsg.Data())).
		Msg("successfully prepared execution data for persistence")

	return err
}

// persistRegisters persists registers from in-memory to permanent storage.
// Registers must be stored for every height, even if it's an empty set.
// No errors are expected during normal operations
func (p *Persister) persistRegisters() error {
	registerData := p.inMemoryRegisters.Data()

	// Always store registers for every height to maintain height continuity
	if err := p.registers.Store(registerData, p.header.Height); err != nil {
		return fmt.Errorf("could not persist registers: %w", err)
	}

	return nil
}

// addEventsToBatch adds events from in-memory storage to the batch.
// No errors are expected during normal operations
func (p *Persister) addEventsToBatch(batch storage.Batch) error {
	if eventsList := p.inMemoryEvents.Data(); len(eventsList) > 0 {
		if err := p.events.BatchStore(p.executionResult.BlockID, []flow.EventsList{eventsList}, batch); err != nil {
			return fmt.Errorf("could not add events to batch: %w", err)
		}
	}

	return nil
}

// addResultsToBatch adds transaction results from in-memory storage to the batch.
// No errors are expected during normal operations
func (p *Persister) addResultsToBatch(batch storage.Batch) error {
	if results := p.inMemoryResults.Data(); len(results) > 0 {
		if err := p.results.BatchStore(p.executionResult.BlockID, results, batch); err != nil {
			return fmt.Errorf("could not add transaction results to batch: %w", err)
		}
	}

	return nil
}

// addCollectionsToBatch persists collections from in-memory to permanent storage.
// No errors are expected during normal operations
func (p *Persister) addCollectionsToBatch(batch storage.Batch) error {
	for _, collection := range p.inMemoryCollections.LightCollections() {
		if err := p.collections.BatchStoreLightAndIndexByTransaction(&collection, batch); err != nil {
			return fmt.Errorf("could not add collections to batch: %w", err)
		}
	}

	return nil
}

// addTransactionsToBatch persists transactions from in-memory to permanent storage.
// No errors are expected during normal operations
func (p *Persister) addTransactionsToBatch(batch storage.Batch) error {
	for _, transaction := range p.inMemoryTransactions.Data() {
		if err := p.transactions.BatchStore(&transaction, batch); err != nil {
			return fmt.Errorf("could not add transactions to batch: %w", err)
		}
	}

	return nil
}

// addTransactionResultErrorMessagesToBatch persists transaction result error messages from in-memory to permanent storage.
// No errors are expected during normal operations
func (p *Persister) addTransactionResultErrorMessagesToBatch(batch storage.Batch) error {
	if txResultErrMsgs := p.inMemoryTxResultErrMsg.Data(); len(txResultErrMsgs) > 0 {
		if err := p.txResultErrMsg.BatchStore(p.header.ID(), txResultErrMsgs, batch); err != nil {
			return fmt.Errorf("could not add transaction result error messages to batch: %w", err)
		}
	}

	return nil
}
