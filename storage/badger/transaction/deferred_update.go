package transaction

import (
	"github.com/dgraph-io/badger/v2"
)

// DeferredDBUpdates is a shorthand notation for a list of anonymous functions that all
// take a `transaction.Tx` as input and run some database operations as part of that transaction.
type DeferredDBUpdates = []DeferredDBUpdate

// DeferredDBUpdate is a shorthand notation for an anonymous function that takes
// a `transaction.Tx` as input and runs some database operations as part of that transaction.
type DeferredDBUpdate = func(tx *Tx) error

// DeferredBadgerUpdates is a shorthand notation for a list of anonymous functions that all
// take a badger transaction as input and run some database operations as part of that transaction.
type DeferredBadgerUpdates = []DeferredBadgerUpdate

// DeferredBadgerUpdate is a shorthand notation for an anonymous function that takes
// a badger transaction as input and runs some database operations as part of that transaction.
type DeferredBadgerUpdate = func(tx *badger.Txn) error

// DeferredDbOps is a utility for accumulating deferred database interactions that
// are supposed to be executed in one atomic transaction. It supports:
//   - Deferred database operations that work directly on Badger transactions.
//   - Deferred database operations that work on `transaction.Tx`.
//     Tx is a storage-layer abstraction, with support for callbacks that are executed
//     after the underlying database transaction completed _successfully_.
//
// Operations and callbacks are executed in the order they are added.
//
// DESIGN PATTERN
//   - DeferredDbOps is stateful, i.e. it needs to be passed as pointer variable.
//   - Do not instantiate Tx outside of this package. Instead, use
//     Update(db, DeferredDbOps.Pending) or View(db, DeferredDbOps.Pending)
//   - Via methods like `AddDbOp`,
type DeferredDbOps struct {
	Pending func(tx *Tx) error
}

func (d *DeferredDbOps) AddBadgerOp(op DeferredBadgerUpdate) *DeferredDbOps {
	prior := d.Pending
	d.Pending = func(tx *Tx) error {
		err := prior(tx)
		if err != nil {
			return err
		}
		err = op(tx.DBTxn)
		if err != nil {
			return err
		}
		return nil
	}
	return d
}

func (d *DeferredDbOps) AddBadgerOps(ops ...DeferredBadgerUpdate) *DeferredDbOps {
	prior := d.Pending
	d.Pending = func(tx *Tx) error {
		err := prior(tx)
		if err != nil {
			return err
		}
		for _, o := range ops {
			err = o(tx.DBTxn)
			if err != nil {
				return err
			}
		}
		return nil
	}
	return d
}

func (d *DeferredDbOps) AddDbOp(op DeferredDBUpdate) *DeferredDbOps {
	prior := d.Pending
	d.Pending = func(tx *Tx) error {
		err := prior(tx)
		if err != nil {
			return err
		}
		err = op(tx)
		if err != nil {
			return err
		}
		return nil
	}
	return d
}

func (d *DeferredDbOps) AddDbOps(ops ...DeferredDBUpdate) *DeferredDbOps {
	prior := d.Pending
	d.Pending = func(tx *Tx) error {
		err := prior(tx)
		if err != nil {
			return err
		}
		for _, o := range ops {
			err = o(tx)
			if err != nil {
				return err
			}
		}
		return nil
	}
	return d
}

// OnSucceed adds a callback to execute after the DeferredDbOps have been successfully
// executed. Useful for pre-emptively adding some element to a cache after the database
// operations has been successfully applied.
func (d *DeferredDbOps) OnSucceed(callback func()) *DeferredDbOps {
	prior := d.Pending
	d.Pending = func(tx *Tx) error {
		err := prior(tx)
		if err != nil {
			return err
		}
		tx.OnSucceed(callback)
		return nil
	}
	return d
}
