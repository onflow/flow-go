package transaction

import (
	"github.com/onflow/flow-go/model/flow"
)

// TODO: maybe move this into the package `storage/badger/transaction` (?)

// DeferredBlockPersistOp is a shorthand notation for an anonymous function that takes the ID of
// a fully constructed block and a `transaction.Tx` as inputs and runs some database operations
// as part of that transaction. It is a "Future Pattern", essentially saying:
// once we have the completed the block's construction, we persist data structures that are
// referenced by the block or populate database indices. This pattern is necessary, because
// internally to the protocol_state package we don't have access to the candidate block ID yet because
// we are still determining the protocol state ID for that block.
type DeferredBlockPersistOp func(blockID flow.Identifier, tx *Tx) error

// noOpPersist intended as constant
var noOpPersist DeferredBlockPersistOp = func(blockID flow.Identifier, tx *Tx) error { return nil }

// WithBlock adds the still missing block ID information to a `DeferredBlockPersistOp`, thereby converting
// it into a `transaction.DeferredDBUpdate`.
func (d DeferredBlockPersistOp) WithBlock(blockID flow.Identifier) DeferredDBUpdate {
	return func(tx *Tx) error {
		return d(blockID, tx)
	}
}

// DeferredBlockPersist is a utility for accumulating deferred database interactions that
// are supposed to be executed in one atomic transaction. It supports:
//   - Deferred database operations that work directly on Badger transactions.
//   - Deferred database operations that work on `transaction.Tx`.
//     Tx is a storage-layer abstraction, with support for callbacks that are executed
//     after the underlying database transaction completed _successfully_.
//   - Deferred database operations that depend on the ID of the block under construction
//     and `transaction.Tx`. Usually, these are operations to populate some `ByBlockID` index.
//
// ORDER OF EXECUTION
// We extend the process in which `transaction.Tx` executes database operations, schedules
// callbacks, and executed the callbacks. Specifically, DeferredDbOps proceeds as follows:
//
//  0. Record functors added via `AddBadgerOp`, `AddDbOp`, `OnSucceed` ...
//     • some functors may schedule callbacks (depending on their type), which are executed
//     after the underlying database transaction completed _successfully_.
//     • `OnSucceed` is treated exactly the same way:
//     it schedules a callback during its execution, but it has no database actions.
//  1. Execute the functors in the order they were added
//  2. During each functor's execution:
//     • some functors may schedule callbacks (depending on their type)
//     • record those callbacks in the order they are scheduled (no execution yet)
//  3. If and only if the underlying database transaction succeeds, run the callbacks
//
// DESIGN PATTERN
//   - DeferredDbOps is stateful, i.e. it needs to be passed as pointer variable.
//   - Do not instantiate Tx directly. Instead, use one of the following
//     transaction.Update(db, DeferredDbOps.Pending().WithBlock(blockID))
//     transaction.View(db, DeferredDbOps.Pending().WithBlock(blockID))
//     operation.RetryOnConflictTx(db, transaction.Update, DeferredDbOps.Pending().WithBlock(blockID))
//
// NOT CONCURRENCY SAFE
type DeferredBlockPersist struct {
	isEmpty bool
	pending DeferredBlockPersistOp
}

// NewDeferredBlockPersist instantiates a DeferredBlockPersist. Initially, it behaves like a no-op until functors are added.
func NewDeferredBlockPersist() *DeferredBlockPersist {
	return &DeferredBlockPersist{
		isEmpty: true,
		pending: noOpPersist, // initially nothing is pending, i.e. no-op
	}
}

// IsEmpty returns true if and only if there are zero pending database operations.
func (d *DeferredBlockPersist) IsEmpty() bool {
	if d == nil {
		return true
	}
	return d.isEmpty
}

// Pending returns a DeferredBlockPersistOp that comprises all database operations and callbacks
// that were added so far. Caution, DeferredBlockPersist keeps its internal state of deferred operations.
// Pending() can be called multiple times, but should only be executed in a database transaction
// once to avoid conflicts.
func (d *DeferredBlockPersist) Pending() DeferredBlockPersistOp {
	if d == nil {
		return noOpPersist
	}
	return d.pending
}

// AddBadgerOp schedules the given DeferredBadgerUpdate to be executed as part of the future transaction.
// For adding multiple DeferredBadgerUpdates, use `AddBadgerOps(ops ...DeferredBadgerUpdate)` if easily possible, as
// it reduces the call stack compared to adding the functors individually via `AddBadgerOp(op DeferredBadgerUpdate)`.
// This method returns a self-reference for chaining.
func (d *DeferredBlockPersist) AddBadgerOp(op DeferredBadgerUpdate) *DeferredBlockPersist {
	prior := d.pending
	d.pending = func(blockID flow.Identifier, tx *Tx) error {
		err := prior(blockID, tx)
		if err != nil {
			return err
		}
		err = op(tx.DBTxn)
		if err != nil {
			return err
		}
		return nil
	}
	d.isEmpty = false
	return d
}

// AddBadgerOps schedules the given DeferredBadgerUpdates to be executed as part of the future transaction.
// This method returns a self-reference for chaining.
func (d *DeferredBlockPersist) AddBadgerOps(ops ...DeferredBadgerUpdate) *DeferredBlockPersist {
	if len(ops) < 1 {
		return d
	}

	prior := d.pending
	d.pending = func(blockID flow.Identifier, tx *Tx) error {
		err := prior(blockID, tx)
		if err != nil {
			return err
		}
		for _, op := range ops {
			err = op(tx.DBTxn)
			if err != nil {
				return err
			}
		}
		return nil
	}
	d.isEmpty = false
	return d
}

// AddDbOp schedules the given DeferredDBUpdate to be executed as part of the future transaction.
// For adding multiple DeferredBadgerUpdates, use `AddDbOps(ops ...DeferredDBUpdate)` if easily possible, as
// it reduces the call stack compared to adding the functors individually via `AddDbOp(op DeferredDBUpdate)`.
// This method returns a self-reference for chaining.
func (d *DeferredBlockPersist) AddDbOp(op DeferredDBUpdate) *DeferredBlockPersist {
	prior := d.pending
	d.pending = func(blockID flow.Identifier, tx *Tx) error {
		err := prior(blockID, tx)
		if err != nil {
			return err
		}
		err = op(tx)
		if err != nil {
			return err
		}
		return nil
	}
	d.isEmpty = false
	return d
}

// AddDbOps schedules the given DeferredDBUpdates to be executed as part of the future transaction.
// This method returns a self-reference for chaining.
func (d *DeferredBlockPersist) AddDbOps(ops ...DeferredDBUpdate) *DeferredBlockPersist {
	if len(ops) < 1 {
		return d
	}

	prior := d.pending
	d.pending = func(blockID flow.Identifier, tx *Tx) error {
		err := prior(blockID, tx)
		if err != nil {
			return err
		}
		for _, op := range ops {
			err = op(tx)
			if err != nil {
				return err
			}
		}
		return nil
	}
	d.isEmpty = false
	return d
}

// AddIndexingOp schedules the given DeferredBlockPersistOp to be executed as part of the future transaction.
// Usually, these are operations to populate some `ByBlockID` index.
// For adding multiple DeferredBlockPersistOps, use `AddIndexingOps(ops ...DeferredBlockPersistOp)` if easily
// possible, as it reduces the call stack compared to adding the functors individually via
// `AddIndexOp(op DeferredBlockPersistOp)`.
// This method returns a self-reference for chaining.
func (d *DeferredBlockPersist) AddIndexingOp(op DeferredBlockPersistOp) *DeferredBlockPersist {
	prior := d.pending
	d.pending = func(blockID flow.Identifier, tx *Tx) error {
		err := prior(blockID, tx)
		if err != nil {
			return err
		}
		err = op(blockID, tx)
		if err != nil {
			return err
		}
		return nil
	}
	d.isEmpty = false
	return d
}

// AddIndexingOps schedules the given DeferredBlockPersistOp to be executed as part of the future transaction.
// Usually, these are operations to populate some `ByBlockID` index.
// This method returns a self-reference for chaining.
func (d *DeferredBlockPersist) AddIndexingOps(ops ...DeferredBlockPersistOp) *DeferredBlockPersist {
	if len(ops) < 1 {
		return d
	}

	prior := d.pending
	d.pending = func(blockID flow.Identifier, tx *Tx) error {
		err := prior(blockID, tx)
		if err != nil {
			return err
		}
		for _, op := range ops {
			err = op(blockID, tx)
			if err != nil {
				return err
			}
		}
		return nil
	}
	d.isEmpty = false
	return d
}

// OnSucceed adds a callback to be executed after the deferred database operations have succeeded. For
// adding multiple callbacks, use `OnSucceeds(callbacks ...func())` if easily possible, as it reduces
// the call stack compared to adding the functors individually via `OnSucceed(callback func())`.
// This method returns a self-reference for chaining.
func (d *DeferredBlockPersist) OnSucceed(callback func()) *DeferredBlockPersist {
	prior := d.pending
	d.pending = func(blockID flow.Identifier, tx *Tx) error {
		err := prior(blockID, tx)
		if err != nil {
			return err
		}
		tx.OnSucceed(callback)
		return nil
	}
	d.isEmpty = false
	return d
}

// OnSucceeds adds callbacks to be executed after the deferred database operations have succeeded.
// This method returns a self-reference for chaining.
func (d *DeferredBlockPersist) OnSucceeds(callbacks ...func()) *DeferredBlockPersist {
	if len(callbacks) < 1 {
		return d
	}

	prior := d.pending
	d.pending = func(blockID flow.Identifier, tx *Tx) error {
		err := prior(blockID, tx)
		if err != nil {
			return err
		}
		for _, c := range callbacks {
			tx.OnSucceed(c)
		}
		return nil
	}
	d.isEmpty = false
	return d
}
