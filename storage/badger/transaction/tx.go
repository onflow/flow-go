package transaction

import (
	dbbadger "github.com/dgraph-io/badger/v2"
	ioutils "github.com/onflow/flow-go/utils/io"
)

type Tx struct {
	DBTxn     *dbbadger.Txn
	callbacks []func()
}

// OnSucceed adds a callback to execute after the batch has
// been successfully flushed.
// useful for implementing the cache where we will only cache
// after the batch has been successfully flushed
func (b *Tx) OnSucceed(callback func()) {
	b.callbacks = append(b.callbacks, callback)
}

// Update creates a badger transaction, passing it to a chain of functions,
// if all succeed. Useful to use callback to update cache in order to ensure data
// in badgerDB and cache are consistent.
func Update(db *dbbadger.DB, f func(*Tx) error) error {
	dbTxn := db.NewTransaction(true)
	defer dbTxn.Discard()

	tx := &Tx{DBTxn: dbTxn}
	err := f(tx)
	if err != nil {
		return err
	}

	err = dbTxn.Commit()
	if err != nil {
		return ioutils.TerminateOnFullDisk(err)
	}

	for _, callback := range tx.callbacks {
		callback()
	}
	return nil
}

// WithTx is useful when transaction is used without adding callback.
func WithTx(f func(*dbbadger.Txn) error) func(*Tx) error {
	return func(tx *Tx) error {
		return f(tx.DBTxn)
	}
}
