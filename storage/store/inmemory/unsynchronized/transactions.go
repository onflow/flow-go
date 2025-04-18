package unsynchronized

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

type Transactions struct {
	//TODO: we don't need a mutex here as we have a guarantee by design
	// that we write data only once and it happens before the future reads.
	// We decided to leave a mutex for some time during active development.
	// It'll be removed later on.
	lock  sync.RWMutex
	store map[flow.Identifier]*flow.TransactionBody
}

var _ storage.Transactions = (*Transactions)(nil)

func NewTransactions() *Transactions {
	return &Transactions{
		store: make(map[flow.Identifier]*flow.TransactionBody),
	}
}

// ByID returns the transaction for the given fingerprint.
// Expected errors during normal operation:
//   - `storage.ErrNotFound` if transaction is not found.
func (t *Transactions) ByID(txID flow.Identifier) (*flow.TransactionBody, error) {
	t.lock.RLock()
	defer t.lock.RUnlock()

	val, ok := t.store[txID]
	if !ok {
		return nil, storage.ErrNotFound
	}

	return val, nil
}

// Store inserts the transaction, keyed by fingerprint. Duplicate transaction insertion is ignored
// No errors are expected during normal operation.
func (t *Transactions) Store(tx *flow.TransactionBody) error {
	t.lock.Lock()
	defer t.lock.Unlock()

	txID := tx.ID()
	if _, ok := t.store[txID]; !ok {
		t.store[txID] = tx
	}

	return nil
}

// AddToBatch adds all the in-memory storages to the given batch.
// It is used for the batching writes to the DB.
func (t *Transactions) AddToBatch(batch storage.ReaderBatchWriter) error {
	writer := batch.Writer()

	for txID, tx := range t.store {
		err := operation.UpsertTransaction(writer, txID, tx)
		if err != nil {
			return fmt.Errorf("could not persist transaction: %w", err)
		}
	}

	return nil
}
