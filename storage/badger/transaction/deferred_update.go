package transaction

import (
	"github.com/dgraph-io/badger/v2"
)

// DeferredDBUpdates is a utility for accumulating deferred database interactions that
// are supposed to be executed in one atomic transaction. It supports
//
// badger transaction and includes and additional slice for callbacks.
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
type DeferredDBUpdates struct {
	deferred func(tx *Tx) error
}

func (d *DeferredDBUpdates) AddBadgerInteraction(update func(*badger.Txn) error) *DeferredDBUpdates {
	prior := d.deferred
	d.deferred = func(tx *Tx) error {
		err := prior(tx)
		if err != nil {
			return err
		}
		err = update(tx.DBTxn)
		if err != nil {
			return err
		}
		return nil
	}
	return d
}

func (d *DeferredDBUpdates) AddBadgerInteractions(updates ...func(*badger.Txn) error) *DeferredDBUpdates {
	prior := d.deferred
	d.deferred = func(tx *Tx) error {
		err := prior(tx)
		if err != nil {
			return err
		}
		for _, u := range updates {
			err = u(tx.DBTxn)
			if err != nil {
				return err
			}
		}
		return nil
	}
	return d
}

func (d *DeferredDBUpdates) AddDbInteraction(update func(*Tx) error) *DeferredDBUpdates {
	prior := d.deferred
	d.deferred = func(tx *Tx) error {
		err := prior(tx)
		if err != nil {
			return err
		}
		err = update(tx)
		if err != nil {
			return err
		}
		return nil
	}
	return d
}

func (d *DeferredDBUpdates) AddDbInteractions(updates ...func(*Tx) error) *DeferredDBUpdates {
	prior := d.deferred
	d.deferred = func(tx *Tx) error {
		err := prior(tx)
		if err != nil {
			return err
		}
		err = update(tx)
		if err != nil {
			return err
		}
		return nil
	}
	return d
}

func (d *DeferredDBUpdates) AddTransactionUpdate(update Tx) *DeferredDBUpdates {
	d.DBTxns = append(d.DBTxns, update.DBTxn)
	return d
}
