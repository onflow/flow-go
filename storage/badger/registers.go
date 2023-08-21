package badger

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

type Registers struct {
	db    *badger.DB
	cache *Cache[ledger.Path, *ledger.Payload]
}

var _ storage.Registers = &Registers{}

func NewRegisters(db *badger.DB) *Registers {
	return &Registers{db: db}
}

func (r *Registers) Store(path ledger.Path, payload *ledger.Payload) error {
	return operation.RetryOnConflictTx(r.db, transaction.Update, r.cache.PutTx(path, payload))
}

func (r *Registers) ByPath(path ledger.Path) (*ledger.Payload, error) {
	tx := r.db.NewTransaction(false)
	defer tx.Discard()
	return r.retrieveRegister(path)(tx)
}

func (r *Registers) retrieveRegister(path ledger.Path) func(tx *badger.Txn) (*ledger.Payload, error) {
	return func(tx *badger.Txn) (*ledger.Payload, error) {
		val, err := r.cache.Get(path)(tx)
		if err != nil {
			return nil, err
		}

		return val, nil
	}
}
