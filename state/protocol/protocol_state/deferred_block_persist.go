package protocol_state

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// DeferredBlockPersistOp is a shorthand notation for an anonymous function that takes
// a fully constructed block and a `transaction.Tx` as inputs and runs some database operations
// as part of that transaction. It is a "Future Pattern", essentially saying:
// once we have the completed the block's construction, we populate core
type DeferredBlockPersistOp func(blockID flow.Identifier, tx *transaction.Tx) error

func (d DeferredBlockPersistOp) WithBlock(blockID flow.Identifier) transaction.DeferredDBUpdate {
	return func(tx *transaction.Tx) error {
		return d(blockID, tx)
	}
}

type DeferredBlockPersist struct {
	pending DeferredBlockPersistOp
}

// NewDeferredBlockPersist instantiates a DeferredBlockPersist. Initially, it behaves like a no-op until functors are added.
func NewDeferredBlockPersist() *DeferredBlockPersist {
	return &DeferredBlockPersist{
		pending: func(flow.Identifier, *transaction.Tx) error { return nil }, // initially nothing is pending, i.e. no-op
	}
}

// Pending returns a DeferredBlockPersistOp that comprises all database operations and callbacks
// that were added so far. Caution, DeferredBlockPersist keeps its internal state of deferred operations.
// Pending() can be called multiple times, but should only be executed in a database transaction
// once to avoid conflicts.
func (d *DeferredBlockPersist) Pending() DeferredBlockPersistOp {
	return d.pending
}

// AddBadgerOp schedules the given DeferredBadgerUpdate to be executed as part of the future transaction.
// For adding multiple DeferredBadgerUpdates, use `AddBadgerOps(ops ...DeferredBadgerUpdate)` if easily possible, as
// it reduces the call stack compared to adding the functors individually via `AddBadgerOp(op DeferredBadgerUpdate)`.
// This method returns a self-reference for chaining.
func (d *DeferredBlockPersist) AddBadgerOp(op transaction.DeferredBadgerUpdate) *DeferredBlockPersist {
	prior := d.pending
	d.pending = func(blockID flow.Identifier, tx *transaction.Tx) error {
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
	return d
}

// AddBadgerOps schedules the given DeferredBadgerUpdates to be executed as part of the future transaction.
// This method returns a self-reference for chaining.
func (d *DeferredBlockPersist) AddBadgerOps(ops ...transaction.DeferredBadgerUpdate) *DeferredBlockPersist {
	if len(ops) < 1 {
		return d
	}

	prior := d.pending
	d.pending = func(blockID flow.Identifier, tx *transaction.Tx) error {
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
	return d
}

// AddDbOp schedules the given DeferredDBUpdate to be executed as part of the future transaction.
// For adding multiple DeferredBadgerUpdates, use `AddDbOps(ops ...DeferredDBUpdate)` if easily possible, as
// it reduces the call stack compared to adding the functors individually via `AddDbOp(op DeferredDBUpdate)`.
// This method returns a self-reference for chaining.
func (d *DeferredBlockPersist) AddDbOp(op transaction.DeferredDBUpdate) *DeferredBlockPersist {
	prior := d.pending
	d.pending = func(blockID flow.Identifier, tx *transaction.Tx) error {
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
	return d
}

// AddDbOps schedules the given DeferredDBUpdates to be executed as part of the future transaction.
// This method returns a self-reference for chaining.
func (d *DeferredBlockPersist) AddDbOps(ops ...transaction.DeferredDBUpdate) *DeferredBlockPersist {
	if len(ops) < 1 {
		return d
	}

	prior := d.pending
	d.pending = func(blockID flow.Identifier, tx *transaction.Tx) error {
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
	d.pending = func(blockID flow.Identifier, tx *transaction.Tx) error {
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
	d.pending = func(blockID flow.Identifier, tx *transaction.Tx) error {
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
	return d
}

// OnSucceed adds a callback to be executed after the deferred database operations have succeeded. For
// adding multiple callbacks, use `OnSucceeds(callbacks ...func())` if easily possible, as it reduces
// the call stack compared to adding the functors individually via `OnSucceed(callback func())`.
// This method returns a self-reference for chaining.
func (d *DeferredBlockPersist) OnSucceed(callback func()) *DeferredBlockPersist {
	prior := d.pending
	d.pending = func(blockID flow.Identifier, tx *transaction.Tx) error {
		err := prior(blockID, tx)
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
func (d *DeferredBlockPersist) OnSucceeds(callbacks ...func()) *DeferredBlockPersist {
	if len(callbacks) < 1 {
		return d
	}

	prior := d.pending
	d.pending = func(blockID flow.Identifier, tx *transaction.Tx) error {
		err := prior(blockID, tx)
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
