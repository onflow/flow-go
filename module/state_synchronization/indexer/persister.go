package indexer

import (
	"fmt"
	"time"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/store/inmemory/unsynchronized"
	"github.com/onflow/flow-go/utils/logging"
	"github.com/rs/zerolog"
)

// BlockPersister handles transferring data from in-memory storages to permanent storages
// for a single block.
//
// - Each BlockPersister instance is created for ONE specific block
// - All `inMemory*` storages contain data ONLY for this specific block, they are not shared
type BlockPersister struct {
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
) *BlockPersister {
	log = log.With().
		Str("component", "block_persister").
		Hex("block_id", logging.ID(executionResult.BlockID)).
		Uint64("height", header.Height).
		Logger()

	persister := &BlockPersister{
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
		Msg("block persister initialized")

	return persister
}

// Persist save data from in-memory storages to the provided persisted storages and commit updates to the database.
// No errors are expected during normal operations
func (p *BlockPersister) Persist() error {
	p.log.Debug().Msg("adding execution data to batch")

	start := time.Now()

	if err := p.persistRegisters(); err != nil {
		return err
	}

	err := p.protocolDB.WithReaderBatchWriter(func(batch storage.ReaderBatchWriter) error {
		if err := p.addEventsToBatch(batch); err != nil {
			return err
		}

		if err := p.addResultsToBatch(batch); err != nil {
			return err
		}

		if err := p.addCollectionsToBatch(batch); err != nil {
			return err
		}

		if err := p.addTransactionsToBatch(batch); err != nil {
			return err
		}

		if err := p.addTransactionResultErrorMessagesToBatch(batch); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		err = fmt.Errorf("failed to commit batch: %w", err)
		return err
	}

	// TODO: include update to latestPersistedSealedResultBlockHeight in the batch

	duration := time.Since(start)

	p.log.Debug().
		Dur("duration_ms", duration).
		Int("events_count", len(p.inMemoryEvents.Data())).
		Int("collection_count", len(p.inMemoryCollections.Data())).
		Int("transactions_count", len(p.inMemoryTransactions.Data())).
		Int("transaction_results_count", len(p.inMemoryResults.Data())).
		Int("transaction_result_error_messages_count", len(p.inMemoryTxResultErrMsg.Data())).
		Msg("successfully prepared execution data for persistence")

	return nil
}

// persistRegisters persists registers from in-memory to permanent storage.
// Registers must be stored for every height, even if it's an empty set.
// No errors are expected during normal operations
func (p *BlockPersister) persistRegisters() error {
	registerData, err := p.inMemoryRegisters.Data(p.header.Height)
	if err != nil {
		return fmt.Errorf("could not get data from registers: %w", err)
	}

	// Always store registers for every height to maintain height continuity
	if err = p.registers.Store(registerData, p.header.Height); err != nil {
		return fmt.Errorf("could not persist registers: %w", err)
	}

	return nil
}

// addEventsToBatch adds events from in-memory storage to the batch.
// No errors are expected during normal operations
func (p *BlockPersister) addEventsToBatch(batch storage.ReaderBatchWriter) error {
	eventsList, err := p.inMemoryEvents.ByBlockID(p.executionResult.BlockID)
	if err != nil {
		return fmt.Errorf("could not get events: %w", err)
	}

	if len(eventsList) > 0 {
		if err := p.events.BatchStore(p.executionResult.BlockID, []flow.EventsList{eventsList}, batch); err != nil {
			return fmt.Errorf("could not add events to batch: %w", err)
		}
	}

	return nil
}

// addResultsToBatch adds transaction results from in-memory storage to the batch.
// No errors are expected during normal operations
func (p *BlockPersister) addResultsToBatch(batch storage.ReaderBatchWriter) error {
	results, err := p.inMemoryResults.ByBlockID(p.executionResult.BlockID)
	if err != nil {
		return fmt.Errorf("could not get results: %w", err)
	}

	if len(results) > 0 {
		if err := p.results.BatchStore(p.executionResult.BlockID, results, batch); err != nil {
			return fmt.Errorf("could not add transaction results to batch: %w", err)
		}
	}

	return nil
}

// addCollectionsToBatch persists collections from in-memory to permanent storage.
// No errors are expected during normal operations
func (p *BlockPersister) addCollectionsToBatch(batch storage.ReaderBatchWriter) error {
	for _, collection := range p.inMemoryCollections.LightCollections() {
		if err := p.collections.BatchStoreLightAndIndexByTransaction(&collection, batch); err != nil {
			return fmt.Errorf("could not add collections to batch: %w", err)
		}
	}

	return nil
}

// addTransactionsToBatch persists transactions from in-memory to permanent storage.
// No errors are expected during normal operations
func (p *BlockPersister) addTransactionsToBatch(batch storage.ReaderBatchWriter) error {
	for _, transaction := range p.inMemoryTransactions.Data() {
		if err := p.transactions.BatchStore(&transaction, batch); err != nil {
			return fmt.Errorf("could not add transactions to batch: %w", err)
		}
	}

	return nil
}

// addTransactionResultErrorMessagesToBatch persists transaction result error messages from in-memory to permanent storage.
// No errors are expected during normal operations
func (p *BlockPersister) addTransactionResultErrorMessagesToBatch(batch storage.ReaderBatchWriter) error {
	txResultErrMsgs, err := p.inMemoryTxResultErrMsg.ByBlockID(p.executionResult.BlockID)
	if err != nil {
		return fmt.Errorf("could not get transaction result error messages: %w", err)
	}

	if len(txResultErrMsgs) > 0 {
		if err := p.txResultErrMsg.BatchStore(p.executionResult.BlockID, txResultErrMsgs, batch); err != nil {
			return fmt.Errorf("could not add transaction result error messages to batch: %w", err)
		}
	}

	return nil
}
