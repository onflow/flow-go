package persisters

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/store/inmemory/unsynchronized"
)

// PersisterStore is the interface to handle persisting of a data type to permanent storage using batch operation.
type PersisterStore interface {
	// Persist adds data to the batch for later commitment.
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

// Persist adds events to the batch.
// No errors are expected during normal operations
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

// Persist adds results to the batch.
// No errors are expected during normal operations
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

var _ PersisterStore = (*LightCollectionsPersister)(nil)

// LightCollectionsPersister handles persisting light collections
type LightCollectionsPersister struct {
	inMemoryCollections  *unsynchronized.Collections
	permanentCollections storage.Collections
}

func NewCollectionsPersister(
	inMemoryCollections *unsynchronized.Collections,
	permanentCollections storage.Collections,
) *LightCollectionsPersister {
	return &LightCollectionsPersister{
		inMemoryCollections:  inMemoryCollections,
		permanentCollections: permanentCollections,
	}
}

// Persist adds light collections to the batch.
// No errors are expected during normal operations
func (c *LightCollectionsPersister) Persist(batch storage.ReaderBatchWriter) error {
	for _, collection := range c.inMemoryCollections.LightCollections() {
		if err := c.permanentCollections.BatchStoreLightAndIndexByTransaction(&collection, batch); err != nil {
			return fmt.Errorf("could not add light collections to batch: %w", err)
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

// Persist adds transactions to the batch.
// No errors are expected during normal operations
func (t *TransactionsPersister) Persist(batch storage.ReaderBatchWriter) error {
	for _, transaction := range t.inMemoryTransactions.Data() {
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

// Persist adds transaction result error messages to the batch.
// No errors are expected during normal operations
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
