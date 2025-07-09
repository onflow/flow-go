package transaction

import (
	dbbadger "github.com/dgraph-io/badger/v2"

	ioutils "github.com/onflow/flow-go/utils/io"
)

// Tx wraps a badger transaction and includes and additional slice for callbacks.
// The callbacks are executed after the badger transaction completed _successfully_.
// DESIGN PATTERN
//   - DBTxn should never be nil
//   - at initialization, `callbacks` is empty
//   - While business logic code operates on `DBTxn`, it can append additional callbacks
//     via the `OnSucceed` method. This generally happens during the transaction execution.
//
// CAUTION:
//   - Tx is stateful (calls to `OnSucceed` change its internal state).
//     Therefore, Tx needs to be passed as pointer variable.
//   - Do not instantiate Tx outside of this package. Instead, use `Update` or `View`
//     functions.
//   - Whether a transaction is considered to have succeeded depends only on the return value
//     of the outermost function. For example, consider a chain of 3 functions: f3( f2( f1(x)))
//     Lets assume f1 fails with an `storage.ErrAlreadyExists` sentinel, which f2 expects and
//     therefore discards. f3 could then succeed, i.e. return nil.
//     Consequently, the entire list of callbacks is executed, including f1's callback if it
//     added one. Callback implementations therefore need to account for this edge case.
//   - not concurrency safe
type Tx struct {
	DBTxn     *dbbadger.Txn
	callbacks []func()
}

// OnSucceed adds a callback to execute after the batch has been successfully flushed.
// Useful for implementing the cache where we will only cache after the batch of database
// operations has been successfully applied.
// CAUTION:
// Whether a transaction is considered to have succeeded depends only on the return value
// of the outermost function. For example, consider a chain of 3 functions: f3( f2( f1(x)))
// Lets assume f1 fails with an `storage.ErrAlreadyExists` sentinel, which f2 expects and
// therefore discards. f3 could then succeed, i.e. return nil.
// Consequently, the entire list of callbacks is executed, including f1's callback if it
// added one. Callback implementations therefore need to account for this edge case.
func (b *Tx) OnSucceed(callback func()) {
	b.callbacks = append(b.callbacks, callback)
}

// Update creates a badger transaction, passing it to a chain of functions.
// Only if transaction succeeds, we run `callbacks` that were appended during the
// transaction execution. The callbacks are useful update caches in order to reduce
// cache misses.
func Update(db *dbbadger.DB, f func(*Tx) error) error {
	dbTxn := db.NewTransaction(true)
	err := run(f, dbTxn)
	dbTxn.Discard()
	return err
}

// View creates a read-only badger transaction, passing it to a chain of functions.
// Only if transaction succeeds, we run `callbacks` that were appended during the
// transaction execution. The callbacks are useful update caches in order to reduce
// cache misses.
func View(db *dbbadger.DB, f func(*Tx) error) error {
	dbTxn := db.NewTransaction(false)
	err := run(f, dbTxn)
	dbTxn.Discard()
	return err
}

func run(f func(*Tx) error, dbTxn *dbbadger.Txn) error {
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
