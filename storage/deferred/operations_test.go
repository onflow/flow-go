package deferred_test

import (
	"errors"
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/deferred"
	"github.com/onflow/flow-go/storage/operation/dbtest"
)

func TestNewDeferredDBOps(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		d := deferred.NewDeferredDBOps()
		assert.NotNil(t, d)
		assert.True(t, d.IsEmpty())
	})
}

func TestDeferredDBOps_IsEmpty(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		d := deferred.NewDeferredDBOps()
		assert.True(t, d.IsEmpty())

		d.AddNextOperation(func(lctx lockctx.Proof, blockID flow.Identifier, rw storage.ReaderBatchWriter) error {
			return nil
		})
		assert.False(t, d.IsEmpty())
	})
}

func TestDeferredDBOps_AddNextOperation_Nil(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		d := deferred.NewDeferredDBOps()
		d.AddNextOperation(nil)
		assert.True(t, d.IsEmpty())
	})
}

func TestDeferredDBOps_Execute_NoOps(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		d := deferred.NewDeferredDBOps()
		err := db.WithReaderBatchWriter(func(writer storage.ReaderBatchWriter) error {
			return d.Execute(nil, flow.Identifier{}, writer)
		})
		assert.NoError(t, err)
	})
}

func TestDeferredDBOps_AddNextOperation_Single(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		d := deferred.NewDeferredDBOps()
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

func TestDeferredDBOps_AddNextOperation_Multiple(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		d := deferred.NewDeferredDBOps()
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

func TestDeferredDBOps_AddNextOperation_Error(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		d := deferred.NewDeferredDBOps()
		var op2Executed bool
		testErr := errors.New("test error")

		op1 := func(lctx lockctx.Proof, blockID flow.Identifier, rw storage.ReaderBatchWriter) error {
			return testErr
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
		assert.Equal(t, testErr, err)
		assert.False(t, op2Executed)
	})
}

func TestDeferredDBOps_Chain(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		d1 := deferred.NewDeferredDBOps()
		var d1op1Executed bool
		d1op1 := func(lctx lockctx.Proof, blockID flow.Identifier, rw storage.ReaderBatchWriter) error {
			d1op1Executed = true
			return nil
		}
		d1.AddNextOperation(d1op1)

		d2 := deferred.NewDeferredDBOps()
		var d2op1Executed bool
		d2op1 := func(lctx lockctx.Proof, blockID flow.Identifier, rw storage.ReaderBatchWriter) error {
			d2op1Executed = true
			return nil
		}
		d2.AddNextOperation(d2op1)

		d1.Chain(d2)

		err := db.WithReaderBatchWriter(func(writer storage.ReaderBatchWriter) error {
			return d1.Execute(nil, flow.Identifier{}, writer)
		})
		require.NoError(t, err)
		assert.True(t, d1op1Executed)
		assert.True(t, d2op1Executed)
	})
}

func TestDeferredDBOps_Chain_Empty(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		d1 := deferred.NewDeferredDBOps()
		var opExecuted bool
		op := func(lctx lockctx.Proof, blockID flow.Identifier, rw storage.ReaderBatchWriter) error {
			opExecuted = true
			return nil
		}
		d1.AddNextOperation(op)

		d2 := deferred.NewDeferredDBOps()
		d1.Chain(d2)

		err := db.WithReaderBatchWriter(func(writer storage.ReaderBatchWriter) error {
			return d1.Execute(nil, flow.Identifier{}, writer)
		})
		require.NoError(t, err)
		assert.True(t, opExecuted)
	})
}

func TestDeferredDBOps_AddSucceedCallback(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		d := deferred.NewDeferredDBOps()
		var callbackExecuted bool
		callback := func() {
			callbackExecuted = true
		}

		d.AddSucceedCallback(callback)

		err := db.WithReaderBatchWriter(func(writer storage.ReaderBatchWriter) error {
			return d.Execute(nil, flow.Identifier{}, writer)
		})
		require.NoError(t, err)
		assert.True(t, callbackExecuted)
	})
}

func TestDeferredDBOps_AddSucceedCallback_Error(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		d := deferred.NewDeferredDBOps()
		var callbackExecuted bool
		callback := func() {
			callbackExecuted = true
		}

		d.AddSucceedCallback(callback)

		testErr := errors.New("test error")
		err := db.WithReaderBatchWriter(func(writer storage.ReaderBatchWriter) error {
			// Execute the deferred operation, which registers the success callback with the writer.
			err := d.Execute(nil, flow.Identifier{}, writer)
			require.NoError(t, err)

			// Return an error from the transaction block to simulate a failed transaction.
			return testErr
		})

		// The error from the transaction should be the one we returned.
		require.Error(t, err)
		assert.Equal(t, testErr, err)

		// Because the transaction failed, the success callback should not have been executed.
		assert.False(t, callbackExecuted)
	})
}

func TestDeferredDBOps_AddSucceedCallback_Chained(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		d := deferred.NewDeferredDBOps()
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
			return d.Execute(nil, flow.Identifier{}, writer)
		})

		require.NoError(t, err)
		assert.True(t, opExecuted)
		assert.True(t, callbackExecuted)
	})
}
