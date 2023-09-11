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

// Fail returns an anonymous function, whose future execution returns the error e. This
// is useful for front-loading sanity checks. On the happy path (dominant), this function
// will generally not be used. However, if one of the front-loaded sanity checks fails,
// we include `transaction.Fail(e)` in place of the business logic handling the happy path.
func Fail(e error) func(*Tx) error {
	return func(tx *Tx) error {
		return e
	}
}

// WithTx is useful when transaction is used without adding callback.
func WithTx(f func(*dbbadger.Txn) error) func(*Tx) error {
	return func(tx *Tx) error {
		return f(tx.DBTxn)
	}
}

// WithBadgerTx adapts a function that takes a *Tx to one that takes a *badger.Txn.
func WithBadgerTx(f func(*Tx) error) func(*dbbadger.Txn) error {
	return func(txn *dbbadger.Txn) error {
		return f(&Tx{DBTxn: txn})
	}
}
