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

func NewTransactions() *Transactions {
	return &Transactions{
		store: make(map[flow.Identifier]*flow.TransactionBody),
	}
}

var _ storage.Transactions = (*Transactions)(nil)

func (t *Transactions) ByID(txID flow.Identifier) (*flow.TransactionBody, error) {
	t.lock.RLock()
	val, ok := t.store[txID]
	t.lock.RUnlock()

	if !ok {
		return nil, storage.ErrNotFound
	}

	return val, nil
}

func (t *Transactions) Store(tx *flow.TransactionBody) error {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.store[tx.ID()] = tx
	return nil
}

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
