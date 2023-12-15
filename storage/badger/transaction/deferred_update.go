package transaction

import (
	"github.com/dgraph-io/badger/v2"
)

// DeferredDBUpdates is a shorthand notation for a list of anonymous functions that all
// take a `transaction.Tx` as input and run some database operations as part of that transaction.
type DeferredDBUpdates = []DeferredDBUpdate

// DeferredDBUpdate is a shorthand notation for an anonymous function that takes
// a `transaction.Tx` as input and runs some database operations as part of that transaction.
type DeferredDBUpdate = func(*Tx) error

// DeferredBadgerUpdates is a shorthand notation for a list of anonymous functions that all
// take a badger transaction as input and run some database operations as part of that transaction.
type DeferredBadgerUpdates = []DeferredBadgerUpdate

// DeferredBadgerUpdate is a shorthand notation for an anonymous function that takes
// a badger transaction as input and runs some database operations as part of that transaction.
type DeferredBadgerUpdate = func(*badger.Txn) error

// DeferredDbOps is a utility for accumulating deferred database interactions that
// are supposed to be executed in one atomic transaction. It supports:
//   - Deferred database operations that work directly on Badger transactions.
//   - Deferred database operations that work on `transaction.Tx`.
//     Tx is a storage-layer abstraction, with support for callbacks that are executed
//     after the underlying database transaction completed _successfully_.
//
// ORDER OF EXECUTION
// We extend the process in which `transaction.Tx` executes database operations, schedules
// callbacks, and executed the callbacks. Specifically, DeferredDbOps proceeds as follows:
//
//  0. Record functors added via `AddBadgerOp`, `AddDbOp`, `OnSucceed` ...
//     • some functor's may schedule callbacks (depending on their type), which are executed
//     after the underlying database transaction completed _successfully_.
//     • `OnSucceed` is treated exactly the same way:
//     it schedules a callback during its execution, but it has no database actions.
//  1. Execute the functors in the order they were added
//  2. During each functor's execution:
//     • some functor's may schedule callbacks (depending on their type)
//     • record those callbacks in the order they are scheduled (no execution yet)
//  3. If and only if the underlying database transaction succeeds, run the callbacks
//
// DESIGN PATTERN
//   - DeferredDbOps is stateful, i.e. it needs to be passed as pointer variable.
//   - Do not instantiate Tx outside of this package. Instead, use
//     Update(db, DeferredDbOps.Pending) or View(db, DeferredDbOps.Pending)
//
// NOT CONCURRENCY SAFE
type DeferredDbOps struct {
	Pending DeferredDBUpdate
}

// NewDeferredDbOps instantiates a DeferredDbOps. Initially, it behaves like a no-op until functors are added.
func NewDeferredDbOps() *DeferredDbOps {
	return &DeferredDbOps{
		Pending: func(tx *Tx) error { return nil }, // initially nothing is pending, i.e. no-op
	}
}

// AddBadgerOp schedules the given DeferredBadgerUpdate to be executed as part of the future transaction.
// For adding multiple DeferredBadgerUpdates, use `AddBadgerOps(ops ...DeferredBadgerUpdate)` if easily possible, as
// it reduces the call stack compared to adding the functors individually via `AddBadgerOp(op DeferredBadgerUpdate)`.
// This method returns a self-reference for chaining.
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

// AddBadgerOps schedules the given DeferredBadgerUpdates to be executed as part of the future transaction.
// This method returns a self-reference for chaining.
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

// AddDbOp schedules the given DeferredDBUpdate to be executed as part of the future transaction.
// For adding multiple DeferredBadgerUpdates, use `AddDbOps(ops ...DeferredDBUpdate)` if easily possible, as
// it reduces the call stack compared to adding the functors individually via `AddDbOp(op DeferredDBUpdate)`.
// This method returns a self-reference for chaining.
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

// AddDbOps schedules the given DeferredDBUpdates to be executed as part of the future transaction.
// This method returns a self-reference for chaining.
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

// OnSucceed adds a callback to be executed after the deferred database operations have succeeded. For
// adding multiple callbacks, use `OnSucceeds(callbacks ...func())` if easily possible, as it reduces
// the call stack compared to adding the functors individually via `OnSucceed(callback func())`.
// This method returns a self-reference for chaining.
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

// OnSucceeds adds callbacks to be executed after the deferred database operations have succeeded.
// This method returns a self-reference for chaining.
func (d *DeferredDbOps) OnSucceeds(callbacks ...func()) *DeferredDbOps {
	prior := d.Pending
	d.Pending = func(tx *Tx) error {
		err := prior(tx)
		if err != nil {
			return err
		}
		for _, c := range callbacks {
			tx.OnSucceed(c)
		}
		return nil
	}
	return d
}
