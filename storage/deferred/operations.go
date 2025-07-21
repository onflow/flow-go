package deferred

import (
	"github.com/jordanschalm/lockctx"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type DBOp = func(lctx lockctx.Proof, blockID flow.Identifier, rw storage.ReaderBatchWriter) error

type DeferredDBOps struct {
	pending DBOp
}

func NewDeferredDBOps() *DeferredDBOps {
	return &DeferredDBOps{
		pending: func(lctx lockctx.Proof, blockID flow.Identifier, rw storage.ReaderBatchWriter) error {
			return nil
		},
	}
}

// AddDbOp schedules the given DeferredDBUpdate to be executed as part of the future transaction.
// it reduces the call stack compared to adding the functors individually via `AddDbOp(op DeferredDBUpdate)`.
func (d *DeferredDBOps) AddNextOperations(nextOperation DBOp) {
	prior := d.pending
	d.pending = func(lctx lockctx.Proof, blockID flow.Identifier, rw storage.ReaderBatchWriter) error {
		// Execute the prior operations first
		if err := prior(lctx, blockID, rw); err != nil {
			return err
		}

		// Then execute the next operation
		if err := nextOperation(lctx, blockID, rw); err != nil {
			return err
		}
		return nil
	}
}

func (d *DeferredDBOps) Chain(deferred *DeferredDBOps) {
	d.AddNextOperations(deferred.pending)
}

// OnSucceeds adds callbacks to be executed after the deferred database operations have succeeded.
func (d *DeferredDBOps) AddSucceedCallback(callback func()) {
	d.AddNextOperations(func(lctx lockctx.Proof, blockID flow.Identifier, rw storage.ReaderBatchWriter) error {
		rw.AddCallback(func(err error) {
			if err == nil {
				callback()
			}
		})
		return nil
	})
}

func (d *DeferredDBOps) Execute(lctx lockctx.Proof, blockID flow.Identifier, rw storage.ReaderBatchWriter) error {
	return d.pending(lctx, blockID, rw)
}
