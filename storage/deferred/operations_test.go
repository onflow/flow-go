package deferred_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/deferred"
	"github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/storage/operation/dbtest"
)

// TestNewDeferredBlockPersist verifies that a newly created DeferredBlockPersist instance is empty and not nil.
func TestNewDeferredBlockPersist(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		d := deferred.NewDeferredBlockPersist()
		assert.NotNil(t, d)
		assert.True(t, d.IsEmpty())
	})
}

// TestDeferredBlockPersist_IsEmpty verifies the working of `DeferredBlockPersist.IsEmpty` method.
func TestDeferredBlockPersist_IsEmpty(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		d := deferred.NewDeferredBlockPersist()
		assert.True(t, d.IsEmpty())

		d.AddNextOperation(func(lctx lockctx.Proof, blockID flow.Identifier, rw storage.ReaderBatchWriter) error {
			return nil
		})
		assert.False(t, d.IsEmpty())
	})
}

// TestDeferredBlockPersist_AddNextOperation_Nil verifies that adding a nil operation does
// not change the state of the DeferredBlockPersist.
func TestDeferredBlockPersist_AddNextOperation_Nil(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		d := deferred.NewDeferredBlockPersist()
		d.AddNextOperation(nil)
		assert.True(t, d.IsEmpty())
	})
}

// TestDeferredBlockPersist_Execute_NoOps verifies that executing an empty DeferredBlockPersist is a no-op.
func TestDeferredBlockPersist_Execute_NoOps(t *testing.T) {
	rw := mock.NewReaderBatchWriter(t) // mock errors on any function call
	d := deferred.NewDeferredBlockPersist()
	err := d.Execute(nil, flow.Identifier{}, rw)
	assert.NoError(t, err)
}

// TestDeferredBlockPersist_AddNextOperation_Single verifies that a single operation can be added and executed.
func TestDeferredBlockPersist_AddNextOperation_Single(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		d := deferred.NewDeferredBlockPersist()
		var executed bool
		op := func(lctx lockctx.Proof, blockID flow.Identifier, rw storage.ReaderBatchWriter) error {
			executed = true
			return nil
		}

		d.AddNextOperation(op)
		err := db.WithReaderBatchWriter(func(writer storage.ReaderBatchWriter) error {
			return d.Execute(nil, flow.Identifier{}, writer)
		})

		require.NoError(t, err)
		assert.True(t, executed)
	})
}

// TestDeferredBlockPersist_AddNextOperation_Multiple verifies that:
//   - multiple operations can be added
//   - operations are executed in the order they were added
func TestDeferredBlockPersist_AddNextOperation_Multiple(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		d := deferred.NewDeferredBlockPersist()
		var executionOrder []int

		op1 := func(lctx lockctx.Proof, blockID flow.Identifier, rw storage.ReaderBatchWriter) error {
			executionOrder = append(executionOrder, 1)
			return nil
		}
		op2 := func(lctx lockctx.Proof, blockID flow.Identifier, rw storage.ReaderBatchWriter) error {
			executionOrder = append(executionOrder, 2)
			return nil
		}

		d.AddNextOperation(op1)
		d.AddNextOperation(op2)

		err := db.WithReaderBatchWriter(func(writer storage.ReaderBatchWriter) error {
			return d.Execute(nil, flow.Identifier{}, writer)
		})

		require.NoError(t, err)
		assert.Equal(t, []int{1, 2}, executionOrder)
	})
}

// TestDeferredBlockPersist_AddNextOperation_Error verifies that if an operation returns an error,
// subsequent operations are not executed and the error is returned.
func TestDeferredBlockPersist_AddNextOperation_Error(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		d := deferred.NewDeferredBlockPersist()
		var op2Executed bool
		testErr := errors.New("test error")

		op1 := func(lctx lockctx.Proof, blockID flow.Identifier, rw storage.ReaderBatchWriter) error {
			return fmt.Errorf("aborting: %w", testErr)
		}
		op2 := func(lctx lockctx.Proof, blockID flow.Identifier, rw storage.ReaderBatchWriter) error {
			op2Executed = true
			return nil
		}

		d.AddNextOperation(op1)
		d.AddNextOperation(op2)

		err := db.WithReaderBatchWriter(func(writer storage.ReaderBatchWriter) error {
			return d.Execute(nil, flow.Identifier{}, writer)
		})

		require.Error(t, err)
		assert.ErrorIs(t, err, testErr)
		assert.False(t, op2Executed)
	})
}

// TestDeferredBlockPersist_Chain verifies that chaining two DeferredBlockPersist:
//   - executes all operations from both instances
//   - maintains the order of operations (first operations from receiver, then from chained instance)
func TestDeferredBlockPersist_Chain(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		var executionOrder []int

		d1 := deferred.NewDeferredBlockPersist()
		d1op1 := func(lctx lockctx.Proof, blockID flow.Identifier, rw storage.ReaderBatchWriter) error {
			executionOrder = append(executionOrder, 1)
			return nil
		}
		d1.AddNextOperation(d1op1)

		d2 := deferred.NewDeferredBlockPersist()
		d2op1 := func(lctx lockctx.Proof, blockID flow.Identifier, rw storage.ReaderBatchWriter) error {
			executionOrder = append(executionOrder, 2)
			return nil
		}
		d2.AddNextOperation(d2op1)

		d1.Chain(d2)

		err := db.WithReaderBatchWriter(func(writer storage.ReaderBatchWriter) error {
			return d1.Execute(nil, flow.Identifier{}, writer)
		})
		require.NoError(t, err)
		assert.Equal(t, []int{1, 2}, executionOrder)
	})
}

// TestDeferredBlockPersist_Chain_Empty verifies that chaining involving an empty DeferredBlockPersist works
func TestDeferredBlockPersist_Chain_Empty(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		t.Run("non-empty receiver chaining an empty DeferredBlockPersist", func(t *testing.T) {
			d := deferred.NewDeferredBlockPersist()
			var opExecuted bool
			op := func(lctx lockctx.Proof, blockID flow.Identifier, rw storage.ReaderBatchWriter) error {
				opExecuted = true
				return nil
			}
			d.AddNextOperation(op)

			empty := deferred.NewDeferredBlockPersist()
			d.Chain(empty)

			err := db.WithReaderBatchWriter(func(writer storage.ReaderBatchWriter) error {
				return d.Execute(nil, flow.Identifier{}, writer)
			})
			require.NoError(t, err)
			assert.True(t, opExecuted)
		})

		t.Run("empty receiver chaining an non-empty DeferredBlockPersist", func(t *testing.T) {
			d := deferred.NewDeferredBlockPersist()
			var opExecuted bool
			op := func(lctx lockctx.Proof, blockID flow.Identifier, rw storage.ReaderBatchWriter) error {
				opExecuted = true
				return nil
			}
			d.AddNextOperation(op)

			empty := deferred.NewDeferredBlockPersist()
			empty.Chain(d)

			err := db.WithReaderBatchWriter(func(writer storage.ReaderBatchWriter) error {
				return empty.Execute(nil, flow.Identifier{}, writer)
			})
			require.NoError(t, err)
			assert.True(t, opExecuted)
		})
	})
}

// TestDeferredBlockPersist_AddSucceedCallback verifies that a callback is executed when commiting the `ReaderBatchWriter`
func TestDeferredBlockPersist_AddSucceedCallback(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		d := deferred.NewDeferredBlockPersist()
		var callbackExecuted bool
		callback := func() {
			callbackExecuted = true
		}
		d.AddSucceedCallback(callback)

		err := db.WithReaderBatchWriter(func(writer storage.ReaderBatchWriter) error {
			// Upon running the deferred operations, the callback should be registered with the writer. However, the
			// callback should not be executed yet, as the writer will only be committed once we return from this function.
			err := d.Execute(nil, flow.Identifier{}, writer)
			require.NoError(t, err)
			assert.False(t, callbackExecuted)
			return nil
		}) // WithReaderBatchWriter commits the batch at the end, which should have triggered the callback
		require.NoError(t, err)
		assert.True(t, callbackExecuted)
	})
}

// TestDeferredBlockPersist_AddSucceedCallback_Error verifies that if an error occurs when committing the batch,
// the success callback is not executed.
func TestDeferredBlockPersist_AddSucceedCallback_Error(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		d := deferred.NewDeferredBlockPersist()
		var callbackExecuted bool
		callback := func() {
			callbackExecuted = true
		}
		d.AddSucceedCallback(callback)

		testErr := errors.New("test error")
		err := db.WithReaderBatchWriter(func(writer storage.ReaderBatchWriter) error {
			// Execute the deferred operation, which registers the success callback with the writer. However, the
			// callback should not be executed yet, as the writer will only be committed once we return from this function.
			err := d.Execute(nil, flow.Identifier{}, writer)
			require.NoError(t, err)
			assert.False(t, callbackExecuted)

			// Return an error from the transaction block to simulate a failed transaction.
			return fmt.Errorf("abort: %w", testErr)
		}) // WithReaderBatchWriter commits the batch at the end, which should have triggered the callback

		// The error from the transaction should be the one we returned.
		require.Error(t, err)
		assert.ErrorIs(t, err, testErr)

		// Because the transaction failed, the success callback should not have been executed.
		assert.False(t, callbackExecuted)
	})
}

// TestDeferredBlockPersist_Add_Operation_and_Callback verifies that
// a deferred operation and a callback can be added
func TestDeferredBlockPersist_Add_Operation_and_Callback(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		d := deferred.NewDeferredBlockPersist()
		var opExecuted bool
		var callbackExecuted bool

		op := func(lctx lockctx.Proof, blockID flow.Identifier, rw storage.ReaderBatchWriter) error {
			opExecuted = true
			return nil
		}
		callback := func() {
			callbackExecuted = true
		}

		d.AddNextOperation(op)
		d.AddSucceedCallback(callback)

		err := db.WithReaderBatchWriter(func(writer storage.ReaderBatchWriter) error {
			// When composing the final write batch, the deferred operations should be run and the callback should
			// be registered with the writer. However, the callback should not be executed yet, as the writer will
			// only be committed once we return from this function.
			err := d.Execute(nil, flow.Identifier{}, writer)
			require.NoError(t, err)
			assert.True(t, opExecuted)
			assert.False(t, callbackExecuted)
			return nil
		}) // WithReaderBatchWriter commits the batch at the end, which should have triggered the callback

		require.NoError(t, err)
		assert.True(t, opExecuted)
		assert.True(t, callbackExecuted)
	})
}
