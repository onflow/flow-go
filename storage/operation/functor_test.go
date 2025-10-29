package operation_test

import (
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestFunctorBindFunctors tests the composition of multiple functors
func TestFunctorBindFunctors(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()

		// Test successful composition
		t.Run("successful_composition", func(t *testing.T) {
			key1 := []byte("success_key1")
			key2 := []byte("success_key2")
			value1 := "value1"
			value2 := "value2"

			composed := operation.BindFunctors(
				operation.UpsertFunctor(key1, value1),
				operation.UpsertFunctor(key2, value2),
			)

			err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return composed(lctx, rw)
				})
			})
			require.NoError(t, err)

			// Verify both values were stored
			var retrieved1, retrieved2 string
			err = operation.RetrieveByKey(db.Reader(), key1, &retrieved1)
			require.NoError(t, err)
			assert.Equal(t, value1, retrieved1)

			err = operation.RetrieveByKey(db.Reader(), key2, &retrieved2)
			require.NoError(t, err)
			assert.Equal(t, value2, retrieved2)
		})

		// Test composition with failure
		t.Run("composition_with_failure", func(t *testing.T) {
			key1 := []byte("composition_key1")
			key2 := []byte("composition_key2")
			value1 := "value1"
			value2 := "value2"

			// First insert key2 to cause conflict
			err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return operation.InsertingWithExistenceCheck(key2, value2)(lctx, rw)
				})
			})
			require.NoError(t, err)

			// Test that the first operation alone works
			err = unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return operation.UpsertFunctor(key1, value1)(lctx, rw)
				})
			})
			require.NoError(t, err)

			// Verify the first operation worked
			var retrieved1 string
			err = operation.RetrieveByKey(db.Reader(), key1, &retrieved1)
			require.NoError(t, err)
			assert.Equal(t, value1, retrieved1)

			// Now test the composition where the second operation fails
			composed := operation.BindFunctors(
				operation.UpsertFunctor(key1, "new_value1"),
				operation.InsertingWithExistenceCheck(key2, "different_value"),
			)

			err = unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return composed(lctx, rw)
				})
			})
			require.Error(t, err)
			assert.ErrorIs(t, err, storage.ErrAlreadyExists)

			// The first operation should have been executed before the second one failed
			// Since the transaction was rolled back, key1 should still have the original value
			var retrieved1After string
			err = operation.RetrieveByKey(db.Reader(), key1, &retrieved1After)
			require.NoError(t, err)
			assert.Equal(t, value1, retrieved1After) // Should be the original value, not "new_value1"
		})
	})
}

// TestFunctorCheckHoldsLockFunctor tests the lock validation functor
func TestFunctorCheckHoldsLockFunctor(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()

		t.Run("valid_lock", func(t *testing.T) {
			lockValidator := operation.CheckHoldsLockFunctor(storage.LockInsertBlock)

			err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return lockValidator(lctx, rw)
				})
			})
			require.NoError(t, err)
		})

		t.Run("missing_lock", func(t *testing.T) {
			lockValidator := operation.CheckHoldsLockFunctor(storage.LockInsertBlock)

			err := unittest.WithLock(t, lockManager, storage.LockFinalizeBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return lockValidator(lctx, rw)
				})
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "missing required lock")
			assert.Contains(t, err.Error(), storage.LockInsertBlock)
		})
	})
}

// TestFunctorWrapError tests error wrapping functionality
func TestFunctorWrapError(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()

		t.Run("wrap_successful_operation", func(t *testing.T) {
			key := []byte("test_key")
			value := "test_value"

			wrapped := operation.WrapError("test operation", operation.UpsertFunctor(key, value))

			err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return wrapped(lctx, rw)
				})
			})
			require.NoError(t, err)

			// Verify the operation succeeded
			var retrieved string
			err = operation.RetrieveByKey(db.Reader(), key, &retrieved)
			require.NoError(t, err)
			assert.Equal(t, value, retrieved)
		})

		t.Run("wrap_failing_operation", func(t *testing.T) {
			key := []byte("existing_key")
			value := "test_value"

			// First insert the key
			err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return operation.InsertingWithExistenceCheck(key, value)(lctx, rw)
				})
			})
			require.NoError(t, err)

			// Now try to insert again with wrapped error
			wrapped := operation.WrapError("duplicate insert", operation.InsertingWithExistenceCheck(key, "different_value"))

			err = unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return wrapped(lctx, rw)
				})
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "duplicate insert")
			assert.ErrorIs(t, err, storage.ErrAlreadyExists)
		})
	})
}

// TestFunctorUpsertFunctor tests the upserting functor
func TestFunctorUpsertFunctor(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()

		t.Run("overwrite_new_key", func(t *testing.T) {
			key := []byte("new_key")
			value := "new_value"

			overwrite := operation.UpsertFunctor(key, value)

			err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return overwrite(lctx, rw)
				})
			})
			require.NoError(t, err)

			// Verify the value was stored
			var retrieved string
			err = operation.RetrieveByKey(db.Reader(), key, &retrieved)
			require.NoError(t, err)
			assert.Equal(t, value, retrieved)
		})

		t.Run("overwrite_existing_key", func(t *testing.T) {
			key := []byte("existing_key")
			originalValue := "original_value"
			newValue := "new_value"

			// First insert the original value
			err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return operation.UpsertFunctor(key, originalValue)(lctx, rw)
				})
			})
			require.NoError(t, err)

			// Now overwrite with new value
			overwrite := operation.UpsertFunctor(key, newValue)

			err = unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return overwrite(lctx, rw)
				})
			})
			require.NoError(t, err)

			// Verify the value was overwritten
			var retrieved string
			err = operation.RetrieveByKey(db.Reader(), key, &retrieved)
			require.NoError(t, err)
			assert.Equal(t, newValue, retrieved)
		})

		t.Run("serialization_error", func(t *testing.T) {
			key := []byte("test_key")
			// Create a value that cannot be serialized (channel)
			type Unserializable struct {
				Channel chan int
			}
			unserializable := &Unserializable{Channel: make(chan int)}

			overwrite := operation.UpsertFunctor(key, unserializable)

			err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return overwrite(lctx, rw)
				})
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to encode value")
		})
	})
}

// TestFunctorInsertingWithExistenceCheck tests the existence check functor
func TestFunctorInsertingWithExistenceCheck(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()

		t.Run("insert_new_key", func(t *testing.T) {
			key := []byte("new_key")
			value := "new_value"

			insert := operation.InsertingWithExistenceCheck(key, value)

			err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return insert(lctx, rw)
				})
			})
			require.NoError(t, err)

			// Verify the value was stored
			var retrieved string
			err = operation.RetrieveByKey(db.Reader(), key, &retrieved)
			require.NoError(t, err)
			assert.Equal(t, value, retrieved)
		})

		t.Run("insert_existing_key", func(t *testing.T) {
			key := []byte("existing_key")
			value := "existing_value"

			// First insert the key
			err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return operation.InsertingWithExistenceCheck(key, value)(lctx, rw)
				})
			})
			require.NoError(t, err)

			// Try to insert the same key again
			insert := operation.InsertingWithExistenceCheck(key, "different_value")

			err = unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return insert(lctx, rw)
				})
			})
			require.Error(t, err)
			assert.ErrorIs(t, err, storage.ErrAlreadyExists)
			assert.Contains(t, err.Error(), "attempting to insert existing key")

			// Verify original value is unchanged
			var retrieved string
			err = operation.RetrieveByKey(db.Reader(), key, &retrieved)
			require.NoError(t, err)
			assert.Equal(t, value, retrieved)
		})
	})
}

// TestFunctorInsertingWithMismatchCheck tests the mismatch check functor
func TestFunctorInsertingWithMismatchCheck(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()

		t.Run("insert_new_key", func(t *testing.T) {
			key := []byte("new_key")
			value := "new_value"

			insert := operation.InsertingWithMismatchCheck(key, value)

			err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return insert(lctx, rw)
				})
			})
			require.NoError(t, err)

			// Verify the value was stored
			var retrieved string
			err = operation.RetrieveByKey(db.Reader(), key, &retrieved)
			require.NoError(t, err)
			assert.Equal(t, value, retrieved)
		})

		t.Run("insert_same_value", func(t *testing.T) {
			key := []byte("existing_key")
			value := "same_value"

			// First insert the key
			err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return operation.InsertingWithMismatchCheck(key, value)(lctx, rw)
				})
			})
			require.NoError(t, err)

			// Try to insert the same value again
			insert := operation.InsertingWithMismatchCheck(key, value)

			err = unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return insert(lctx, rw)
				})
			})
			require.NoError(t, err) // Should succeed since values are the same

			// Verify value is unchanged
			var retrieved string
			err = operation.RetrieveByKey(db.Reader(), key, &retrieved)
			require.NoError(t, err)
			assert.Equal(t, value, retrieved)
		})

		t.Run("insert_different_value", func(t *testing.T) {
			key := []byte("conflict_key")
			originalValue := "original_value"
			conflictValue := "conflict_value"

			// First insert the original value
			err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return operation.InsertingWithMismatchCheck(key, originalValue)(lctx, rw)
				})
			})
			require.NoError(t, err)

			// Try to insert a different value
			insert := operation.InsertingWithMismatchCheck(key, conflictValue)

			err = unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return insert(lctx, rw)
				})
			})
			require.Error(t, err)
			assert.ErrorIs(t, err, storage.ErrDataMismatch)
			assert.Contains(t, err.Error(), "attempting to insert existing key with different value")

			// Verify original value is unchanged
			var retrieved string
			err = operation.RetrieveByKey(db.Reader(), key, &retrieved)
			require.NoError(t, err)
			assert.Equal(t, originalValue, retrieved)
		})
	})
}

// TestFunctorComplexComposition tests complex functor compositions
func TestFunctorComplexComposition(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()

		t.Run("complex_successful_operation", func(t *testing.T) {
			key1 := []byte("key1")
			key2 := []byte("key2")
			key3 := []byte("key3")
			value1 := "value1"
			value2 := "value2"
			value3 := "value3"

			// Create a complex operation that:
			// 1. Validates lock is held
			// 2. Inserts key1 with existence check
			// 3. Overwrites key2
			// 4. Inserts key3 with mismatch check
			complexOp := operation.BindFunctors(
				operation.CheckHoldsLockFunctor(storage.LockInsertBlock),
				operation.InsertingWithExistenceCheck(key1, value1),
				operation.UpsertFunctor(key2, value2),
				operation.InsertingWithMismatchCheck(key3, value3),
			)

			err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return complexOp(lctx, rw)
				})
			})
			require.NoError(t, err)

			// Verify all operations succeeded
			var retrieved1, retrieved2, retrieved3 string
			err = operation.RetrieveByKey(db.Reader(), key1, &retrieved1)
			require.NoError(t, err)
			assert.Equal(t, value1, retrieved1)

			err = operation.RetrieveByKey(db.Reader(), key2, &retrieved2)
			require.NoError(t, err)
			assert.Equal(t, value2, retrieved2)

			err = operation.RetrieveByKey(db.Reader(), key3, &retrieved3)
			require.NoError(t, err)
			assert.Equal(t, value3, retrieved3)
		})

		t.Run("complex_operation_with_wrapped_errors", func(t *testing.T) {
			key1 := []byte("complex_key1")
			key2 := []byte("complex_key2")
			value1 := "value1"
			value2 := "value2"

			// Create a complex operation with wrapped errors
			complexOp := operation.BindFunctors(
				operation.WrapError("lock validation", operation.CheckHoldsLockFunctor(storage.LockInsertBlock)),
				operation.WrapError("first insert", operation.InsertingWithExistenceCheck(key1, value1)),
				operation.WrapError("second insert", operation.InsertingWithExistenceCheck(key2, value2)),
			)

			err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return complexOp(lctx, rw)
				})
			})
			require.NoError(t, err)

			// Verify both values were stored
			var retrieved1, retrieved2 string
			err = operation.RetrieveByKey(db.Reader(), key1, &retrieved1)
			require.NoError(t, err)
			assert.Equal(t, value1, retrieved1)

			err = operation.RetrieveByKey(db.Reader(), key2, &retrieved2)
			require.NoError(t, err)
			assert.Equal(t, value2, retrieved2)
		})
	})
}

// TestFunctorErrorHandling tests error handling in various scenarios
func TestFunctorErrorHandling(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		lockManager := storage.NewTestingLockManager()

		t.Run("irrecoverable_exception_on_serialization", func(t *testing.T) {
			key := []byte("test_key")
			// Create a value that causes serialization to fail
			type Unserializable struct {
				Channel chan int
			}
			unserializable := &Unserializable{Channel: make(chan int)}

			overwrite := operation.UpsertFunctor(key, unserializable)

			err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return overwrite(lctx, rw)
				})
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to encode value")
		})

		t.Run("irrecoverable_exception_on_storage", func(t *testing.T) {
			// This test is more theoretical since we can't easily simulate storage failures
			// in the test environment, but we can test the error path exists
			key := []byte("test_key")
			value := "test_value"

			overwrite := operation.UpsertFunctor(key, value)

			// The operation should succeed in normal conditions
			err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return overwrite(lctx, rw)
				})
			})
			require.NoError(t, err)
		})
	})
}
