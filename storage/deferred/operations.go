package deferred

import (
	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// DBOp is a shorthand for a deferred database operation that works within a lock-protected context.
// It accepts a lock proof, a block ID, and a reader/writer interface to perform its task.
// This pattern allows chaining database updates for atomic execution in one batch updates.
type DBOp = func(lctx lockctx.Proof, blockID flow.Identifier, rw storage.ReaderBatchWriter) error

// DeferredBlockPersist accumulates deferred database operations to be executed later in a single atomic batch update.
// Specifically, we defer appending writes and success-callbacks to a [storage.ReaderBatchWriter].
// Operations for appending writes and success-callbacks are executed in the order in which they were queued.
// Since Pebble does not provide serializable snapshot isolation, callers MUST ensure that the necessary locks are
// acquired before executing the set of deferred operations.
//
// This construct accomplishes two distinct goals:
//  1. Deferring block indexing write operations when the block ID is not yet known.
//  2. Deferring lock-requiring read-then-write operations to minimize time spent holding a lock.
//
// NOT CONCURRENCY SAFE
type DeferredBlockPersist struct {
	pending DBOp // Holds the accumulated operations as a single composed function. Can be nil if no ops are added.
}

// NewDeferredBlockPersist instantiates a DeferredBlockPersist instance. Initially, it behaves as a no-op until operations are added.
func NewDeferredBlockPersist() *DeferredBlockPersist {
	return &DeferredBlockPersist{
		pending: nil,
	}
}

// IsEmpty returns true if no operations have been enqueued.
func (d *DeferredBlockPersist) IsEmpty() bool {
	return d.pending == nil
}

// AddNextOperation adds a new deferred database operation to the queue of pending operations.
// If there are already pending operations, this new operation will be composed to run after them.
// This method ensures the operations execute sequentially and abort on the first error.
//
// If `nil` is passed, it is ignored — this might happen if chaining with an empty DeferredBlockPersist.
func (d *DeferredBlockPersist) AddNextOperation(nextOperation DBOp) {
	if nextOperation == nil {
		// No-op if the provided operation is nil.
		return
	}

	if d.pending == nil {
		// If this is the first operation being added, set it directly.
		d.pending = nextOperation
		return
	}

	// Compose the prior and next operations into a single function.
	prior := d.pending
	d.pending = func(lctx lockctx.Proof, blockID flow.Identifier, rw storage.ReaderBatchWriter) error {
		// Execute the prior operations first.
		if err := prior(lctx, blockID, rw); err != nil {
			return err
		}
		// Execute the newly added operation next.
		if err := nextOperation(lctx, blockID, rw); err != nil {
			return err
		}
		return nil
	}
}

// Chain merges the deferred operations from another DeferredBlockPersist into this one.
// The resulting order of operations is:
// 1. execute the operations in the receiver in the order they were added
// 2. execute the operations from the input in the order they were added
func (d *DeferredBlockPersist) Chain(deferred *DeferredBlockPersist) {
	d.AddNextOperation(deferred.pending)
}

// AddSucceedCallback adds a callback to be executed **after** the pending database operations succeed.
// This is useful for registering indexing tasks or post-commit hooks.
// The callback is only invoked if no error occurred during batch updates execution.
func (d *DeferredBlockPersist) AddSucceedCallback(callback func()) {
	d.AddNextOperation(func(_ lockctx.Proof, blockID flow.Identifier, rw storage.ReaderBatchWriter) error {
		// Schedule the callback to run after a successful commit.
		storage.OnCommitSucceed(rw, callback)
		return nil
	})
}

// Execute runs all the accumulated deferred database operations in-order.
// If no operations were added, it is effectively a no-op.
// This method should be called exactly once per batch updates context.
func (d *DeferredBlockPersist) Execute(lctx lockctx.Proof, blockID flow.Identifier, rw storage.ReaderBatchWriter) error {
	if d.pending == nil {
		return nil // No operations to execute.
	}
	return d.pending(lctx, blockID, rw)
}
