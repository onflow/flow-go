package operation_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

func InsertNewEntity(lock *sync.Mutex, rw storage.ReaderBatchWriter, e Entity) error {
	rw.Lock(lock)

	var item Entity
	err := operation.Retrieve(e.Key(), &item)(rw.GlobalReader())
	if err == nil {
		return storage.ErrAlreadyExists
	}

	if !errors.Is(err, storage.ErrNotFound) {
		return err
	}

	return operation.UpsertByKey(rw.Writer(), e.Key(), e)
}

func TestLockReEntrance(t *testing.T) {
	t.Parallel()

	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		inserting := sync.Mutex{}
		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			err := InsertNewEntity(&inserting, rw, Entity{ID: 1})
			if err != nil {
				return err
			}
			// Re-entrant call to InsertNewEntity
			err = InsertNewEntity(&inserting, rw, Entity{ID: 2})
			if err != nil {
				return err
			}

			return nil
		}))

		var item Entity
		require.NoError(t, operation.Retrieve(Entity{ID: 1}.Key(), &item)(db.Reader()))
		require.Equal(t, Entity{ID: 1}, item)

		require.NoError(t, operation.Retrieve(Entity{ID: 2}.Key(), &item)(db.Reader()))
		require.Equal(t, Entity{ID: 2}, item)
	})
}

func TestLockSeqential(t *testing.T) {
	t.Parallel()

	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		inserting := sync.Mutex{}
		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return InsertNewEntity(&inserting, rw, Entity{ID: 1})
		}))

		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return InsertNewEntity(&inserting, rw, Entity{ID: 2})
		}))

		var item Entity
		require.NoError(t, operation.Retrieve(Entity{ID: 1}.Key(), &item)(db.Reader()))
		require.Equal(t, Entity{ID: 1}, item)

		require.NoError(t, operation.Retrieve(Entity{ID: 2}.Key(), &item)(db.Reader()))
		require.Equal(t, Entity{ID: 2}, item)
	})
}

func TestLockConcurrentInsert(t *testing.T) {
	t.Parallel()

	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		var (
			wg    sync.WaitGroup
			lock  sync.Mutex
			count = 10 // number of concurrent inserts
		)

		entities := make([]Entity, count)
		for i := 0; i < count; i++ {
			entities[i] = Entity{ID: uint64(i)}
		}

		wg.Add(count)
		for i := 0; i < count; i++ {
			i := i // capture loop variable
			go func() {
				defer wg.Done()
				err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return InsertNewEntity(&lock, rw, entities[i])
				})
				require.NoError(t, err)
			}()
		}

		wg.Wait()

		// Verify all entities were inserted correctly
		for i := 0; i < count; i++ {
			var result Entity
			err := operation.Retrieve(entities[i].Key(), &result)(db.Reader())
			require.NoError(t, err)
			require.Equal(t, entities[i], result)
		}
	})
}

// concurrently inserting the same entity 10 times, should only succeed 1 time,
// and fail 9 times
func TestLockConcurrentInsertError(t *testing.T) {
	t.Parallel()

	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		var (
			wg    sync.WaitGroup
			lock  sync.Mutex
			count = 10 // number of concurrent inserts
		)

		entity := Entity{ID: uint64(1)}
		failedCount := atomic.NewInt32(0)

		wg.Add(count)
		for i := 0; i < count; i++ {
			go func() {
				defer wg.Done()
				err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return InsertNewEntity(&lock, rw, entity)
				})

				if err != nil {
					failedCount.Add(1)
				}
			}()
		}

		wg.Wait()

		// Verify the entity was inserted correctly
		var result Entity
		err := operation.Retrieve(entity.Key(), &result)(db.Reader())
		require.NoError(t, err)
		require.Equal(t, entity, result)

		// and failed 9 times
		require.Equal(t, int32(9), failedCount.Load(), "expected 9 failed inserts")
	})
}
