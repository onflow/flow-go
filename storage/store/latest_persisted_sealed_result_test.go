package store

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/cockroachdb/pebble/v2"
	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestNewLatestPersistedSealedResult tests the initialization of LatestPersistedSealedResult.
// It verifies that:
// - The ConsumerProgress is properly stored
// - All fields are correctly initialized on success
func TestNewLatestPersistedSealedResult(t *testing.T) {
	initialHeight := uint64(100)
	missingHeaderHeight := initialHeight + 1
	missingResultHeight := initialHeight + 2

	initialHeader, initialResult, mockHeaders, mockResults := getHeadersResults(t, initialHeight)

	unittest.RunWithPebbleDB(t, func(pdb *pebble.DB) {
		db := pebbleimpl.ToDB(pdb)

		t.Run("successful initialization", func(t *testing.T) {
			initializer := NewConsumerProgress(db, "test_consumer1")
			progress, err := initializer.Initialize(initialHeight)
			require.NoError(t, err)

			latest, err := NewLatestPersistedSealedResult(progress, mockHeaders, mockResults)
			require.NoError(t, err)

			require.NotNil(t, latest)

			actualResultID, actualHeight := latest.Latest()

			assert.Equal(t, initialResult.ID(), actualResultID)
			assert.Equal(t, initialHeader.Height, actualHeight)
		})

		t.Run("processed index error", func(t *testing.T) {
			expectedErr := fmt.Errorf("processed index error")

			mockCP := storagemock.NewConsumerProgress(t)
			mockCP.On("ProcessedIndex").Return(uint64(0), expectedErr)

			latest, err := NewLatestPersistedSealedResult(mockCP, nil, nil)

			assert.ErrorIs(t, err, expectedErr)
			require.Nil(t, latest)
		})

		t.Run("header lookup error", func(t *testing.T) {
			expectedErr := fmt.Errorf("header lookup error")

			initializer := NewConsumerProgress(db, "test_consumer2")
			progress, err := initializer.Initialize(missingHeaderHeight)
			require.NoError(t, err)

			mockHeaders.On("ByHeight", missingHeaderHeight).Return(nil, expectedErr)

			latest, err := NewLatestPersistedSealedResult(progress, mockHeaders, nil)

			assert.ErrorIs(t, err, expectedErr)
			require.Nil(t, latest)
		})

		t.Run("result lookup error", func(t *testing.T) {
			expectedErr := fmt.Errorf("result lookup error")

			initializer := NewConsumerProgress(db, "test_consumer3")
			progress, err := initializer.Initialize(missingResultHeight)
			require.NoError(t, err)

			header := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(missingResultHeight))

			mockHeaders.On("ByHeight", missingResultHeight).Return(header, nil)
			mockResults.On("ByBlockID", header.ID()).Return(nil, expectedErr)

			latest, err := NewLatestPersistedSealedResult(progress, mockHeaders, mockResults)

			assert.ErrorIs(t, err, expectedErr)
			require.Nil(t, latest)
		})
	})

}

// TestLatestPersistedSealedResult_BatchSet tests the batch update functionality.
// It verifies that:
// - Updates are atomic - either all state is updated or none
// - The callback mechanism works correctly for both success and failure cases
// - State is not updated if BatchSetProcessedIndex fails
// - State is only updated after the batch callback indicates success
func TestLatestPersistedSealedResult_BatchSet(t *testing.T) {
	initialHeader, initialResult, mockHeaders, mockResults := getHeadersResults(t, 100)

	newResultID := unittest.IdentifierFixture()
	newHeight := uint64(200)

	unittest.RunWithPebbleDB(t, func(pdb *pebble.DB) {
		db := pebbleimpl.ToDB(pdb)

		t.Run("successful batch update", func(t *testing.T) {
			lockManager := storage.NewTestingLockManager()

			initializer := NewConsumerProgress(db, "test_consumer1")
			progress, err := initializer.Initialize(initialHeader.Height)
			require.NoError(t, err)

			latest, err := NewLatestPersistedSealedResult(progress, mockHeaders, mockResults)
			require.NoError(t, err)

			err = unittest.WithLock(t, lockManager, storage.LockUpdateLatestPersistedSealedResult, func(lctx lockctx.Context) error {
				done := make(chan struct{})
				err := db.WithReaderBatchWriter(func(rbw storage.ReaderBatchWriter) error {
					rbw.AddCallback(func(err error) {
						require.NoError(t, err)
						close(done)
					})

					return latest.BatchSet(lctx, newResultID, newHeight, rbw)
				})
				require.NoError(t, err)

				unittest.RequireCloseBefore(t, done, 100*time.Millisecond, "callback not called")

				actualResultID, actualHeight := latest.Latest()

				assert.Equal(t, newResultID, actualResultID)
				assert.Equal(t, newHeight, actualHeight)

				return nil
			})
			require.NoError(t, err)
		})
	})

	t.Run("batch update error during BatchSetProcessedIndex", func(t *testing.T) {
		lockManager := storage.NewTestingLockManager()
		expectedErr := fmt.Errorf("could not set processed index")

		var callbackCalled sync.WaitGroup
		callbackCalled.Add(1)

		mockBatch := storagemock.NewReaderBatchWriter(t)
		mockBatch.On("AddCallback", mock.AnythingOfType("func(error)")).Run(func(args mock.Arguments) {
			callback := args.Get(0).(func(error))
			callback(expectedErr)
			callbackCalled.Done()
		})

		mockCP := storagemock.NewConsumerProgress(t)
		mockCP.On("ProcessedIndex").Return(initialHeader.Height, nil)
		mockCP.On("BatchSetProcessedIndex", newHeight, mockBatch).Return(expectedErr)

		latest, err := NewLatestPersistedSealedResult(mockCP, mockHeaders, mockResults)
		require.NoError(t, err)

		err = unittest.WithLock(t, lockManager, storage.LockUpdateLatestPersistedSealedResult, func(lctx lockctx.Context) error {
			return latest.BatchSet(lctx, newResultID, newHeight, mockBatch)
		})
		assert.ErrorIs(t, err, expectedErr)

		callbackCalled.Wait()

		actualResultID, actualHeight := latest.Latest()

		assert.Equal(t, initialResult.ID(), actualResultID)
		assert.Equal(t, initialHeader.Height, actualHeight)
	})
}

// TestLatestPersistedSealedResult_ConcurrentAccess tests the thread safety of the implementation.
// It verifies that:
// - Multiple concurrent reads are safe
// - Concurrent reads and writes are properly synchronized
// - No data races occur under heavy concurrent load
// - The state remains consistent during concurrent operations
func TestLatestPersistedSealedResult_ConcurrentAccess(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(pdb *pebble.DB) {
		db := pebbleimpl.ToDB(pdb)

		initialHeader, initialResult, mockHeaders, mockResults := getHeadersResults(t, 100)

		initializer := NewConsumerProgress(db, "test_consumer")
		progress, err := initializer.Initialize(initialHeader.Height)
		require.NoError(t, err)

		latest, err := NewLatestPersistedSealedResult(progress, mockHeaders, mockResults)
		require.NoError(t, err)

		t.Run("concurrent reads", func(t *testing.T) {
			var wg sync.WaitGroup
			numGoroutines := 1000

			for range numGoroutines {
				wg.Add(1)
				go func() {
					defer wg.Done()

					actualResultID, actualHeight := latest.Latest()

					assert.Equal(t, initialResult.ID(), actualResultID)
					assert.Equal(t, initialHeader.Height, actualHeight)
				}()
			}

			wg.Wait()
		})

		t.Run("concurrent read/write", func(t *testing.T) {
			lockManager := storage.NewTestingLockManager()
			var wg sync.WaitGroup
			numGoroutines := 1000

			for i := 0; i < numGoroutines; i++ {
				wg.Add(2)
				go func(i int) {
					defer wg.Done()

					err := unittest.WithLock(t, lockManager, storage.LockUpdateLatestPersistedSealedResult, func(lctx lockctx.Context) error {
						return db.WithReaderBatchWriter(func(rbw storage.ReaderBatchWriter) error {
							newResultID := unittest.IdentifierFixture()
							newHeight := uint64(200 + i)
							return latest.BatchSet(lctx, newResultID, newHeight, rbw)
						})
					})
					require.NoError(t, err)
				}(i)
				go func() {
					defer wg.Done()
					_, _ = latest.Latest()
				}()
			}

			wg.Wait()
		})
	})
}

func getHeadersResults(t *testing.T, initialHeight uint64) (*flow.Header, *flow.ExecutionResult, *storagemock.Headers, *storagemock.ExecutionResults) {
	header := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(initialHeight))
	result := unittest.ExecutionResultFixture(func(result *flow.ExecutionResult) {
		result.BlockID = header.ID()
	})

	mockHeaders := storagemock.NewHeaders(t)
	mockHeaders.On("ByHeight", initialHeight).Return(header, nil).Maybe()

	mockResults := storagemock.NewExecutionResults(t)
	mockResults.On("ByBlockID", result.BlockID).Return(result, nil).Maybe()

	return header, result, mockHeaders, mockResults
}
