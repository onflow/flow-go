package pipeline

import (
	"fmt"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/store/inmemory/unsynchronized"
)

// PersisterStore is the interface for all data type persisters that use batch operations.
// Each implementation handles persistence of a specific data type (events, results, collections, etc.)
// to permanent storage using batch operations for the efficient database writes.
type PersisterStore interface {
	// Persist adds this data type to the batch for later commitment.
	// No errors are expected during normal operations
	Persist(batch storage.ReaderBatchWriter) error
}

var _ PersisterStore = (*EventsPersister)(nil)

// EventsPersister handles persisting events
type EventsPersister struct {
	inMemoryEvents  *unsynchronized.Events
	permanentEvents storage.Events
	blockID         flow.Identifier
}

func NewEventsPersister(
	inMemoryEvents *unsynchronized.Events,
	permanentEvents storage.Events,
	blockID flow.Identifier,
) *EventsPersister {
	return &EventsPersister{
		inMemoryEvents:  inMemoryEvents,
		permanentEvents: permanentEvents,
		blockID:         blockID,
	}
}

func (e *EventsPersister) Persist(batch storage.ReaderBatchWriter) error {
	eventsList, err := e.inMemoryEvents.ByBlockID(e.blockID)
	if err != nil {
		return fmt.Errorf("could not get events: %w", err)
	}

	if len(eventsList) > 0 {
		if err := e.permanentEvents.BatchStore(e.blockID, []flow.EventsList{eventsList}, batch); err != nil {
			return fmt.Errorf("could not add events to batch: %w", err)
		}
	}

	return nil
}

var _ PersisterStore = (*ResultsPersister)(nil)

// ResultsPersister handles persisting transaction results
type ResultsPersister struct {
	inMemoryResults  *unsynchronized.LightTransactionResults
	permanentResults storage.LightTransactionResults
	blockID          flow.Identifier
}

func NewResultsPersister(
	inMemoryResults *unsynchronized.LightTransactionResults,
	permanentResults storage.LightTransactionResults,
	blockID flow.Identifier,
) *ResultsPersister {
	return &ResultsPersister{
		inMemoryResults:  inMemoryResults,
		permanentResults: permanentResults,
		blockID:          blockID,
	}
}

func (r *ResultsPersister) Persist(batch storage.ReaderBatchWriter) error {
	results, err := r.inMemoryResults.ByBlockID(r.blockID)
	if err != nil {
		return fmt.Errorf("could not get results: %w", err)
	}

	if len(results) > 0 {
		if err := r.permanentResults.BatchStore(r.blockID, results, batch); err != nil {
			return fmt.Errorf("could not add transaction results to batch: %w", err)
		}
	}

	return nil
}

var _ PersisterStore = (*CollectionsPersister)(nil)

// CollectionsPersister handles persisting collections
type CollectionsPersister struct {
	inMemoryCollections  *unsynchronized.Collections
	permanentCollections storage.Collections
}

func NewCollectionsPersister(
	inMemoryCollections *unsynchronized.Collections,
	permanentCollections storage.Collections,
) *CollectionsPersister {
	return &CollectionsPersister{
		inMemoryCollections:  inMemoryCollections,
		permanentCollections: permanentCollections,
	}
}

func (c *CollectionsPersister) Persist(batch storage.ReaderBatchWriter) error {
	collections := c.inMemoryCollections.LightCollections()

	if len(collections) == 0 {
		return nil
	}

	for _, collection := range collections {
		if err := c.permanentCollections.BatchStoreLightAndIndexByTransaction(&collection, batch); err != nil {
			return fmt.Errorf("could not add collections to batch: %w", err)
		}
	}

	return nil
}

var _ PersisterStore = (*TransactionsPersister)(nil)

// TransactionsPersister handles persisting transactions
type TransactionsPersister struct {
	inMemoryTransactions  *unsynchronized.Transactions
	permanentTransactions storage.Transactions
}

func NewTransactionsPersister(
	inMemoryTransactions *unsynchronized.Transactions,
	permanentTransactions storage.Transactions,
) *TransactionsPersister {
	return &TransactionsPersister{
		inMemoryTransactions:  inMemoryTransactions,
		permanentTransactions: permanentTransactions,
	}
}

func (t *TransactionsPersister) Persist(batch storage.ReaderBatchWriter) error {
	transactions := t.inMemoryTransactions.Data()

	if len(transactions) == 0 {
		return nil
	}

	for _, transaction := range transactions {
		if err := t.permanentTransactions.BatchStore(&transaction, batch); err != nil {
			return fmt.Errorf("could not add transactions to batch: %w", err)
		}
	}

	return nil
}

var _ PersisterStore = (*TxResultErrMsgPersister)(nil)

// TxResultErrMsgPersister handles persisting transaction result error messages
type TxResultErrMsgPersister struct {
	inMemoryTxResultErrMsg  *unsynchronized.TransactionResultErrorMessages
	permanentTxResultErrMsg storage.TransactionResultErrorMessages
	blockID                 flow.Identifier
}

func NewTxResultErrMsgPersister(
	inMemoryTxResultErrMsg *unsynchronized.TransactionResultErrorMessages,
	permanentTxResultErrMsg storage.TransactionResultErrorMessages,
	blockID flow.Identifier,
) *TxResultErrMsgPersister {
	return &TxResultErrMsgPersister{
		inMemoryTxResultErrMsg:  inMemoryTxResultErrMsg,
		permanentTxResultErrMsg: permanentTxResultErrMsg,
		blockID:                 blockID,
	}
}

func (t *TxResultErrMsgPersister) Persist(batch storage.ReaderBatchWriter) error {
	txResultErrMsgs, err := t.inMemoryTxResultErrMsg.ByBlockID(t.blockID)
	if err != nil {
		return fmt.Errorf("could not get transaction result error messages: %w", err)
	}

	if len(txResultErrMsgs) > 0 {
		if err := t.permanentTxResultErrMsg.BatchStore(t.blockID, txResultErrMsgs, batch); err != nil {
			return fmt.Errorf("could not add transaction result error messages to batch: %w", err)
		}
	}

	return nil
}

// RegistersPersister handles registers
type RegistersPersister struct {
	inMemoryRegisters  *unsynchronized.Registers
	permanentRegisters storage.RegisterIndex
	height             uint64
}

func NewRegistersPersister(
	inMemoryRegisters *unsynchronized.Registers,
	permanentRegisters storage.RegisterIndex,
	height uint64,
) *RegistersPersister {
	return &RegistersPersister{
		inMemoryRegisters:  inMemoryRegisters,
		permanentRegisters: permanentRegisters,
		height:             height,
	}
}

// PersistDirectly persists registers directly (not using batch since it's a different DB)
func (r *RegistersPersister) PersistDirectly() error {
	registerData, err := r.inMemoryRegisters.Data(r.height)
	if err != nil {
		return fmt.Errorf("could not get data from registers: %w", err)
	}

	if err := r.permanentRegisters.Store(registerData, r.height); err != nil {
		return fmt.Errorf("could not persist registers: %w", err)
	}

	return nil
}
