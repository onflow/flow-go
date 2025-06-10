package ingestion2

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestNewLatestPersistedSealedResult tests the initialization of LatestPersistedSealedResult.
// It verifies that:
// - The initializer is called with the correct height
// - The returned ConsumerProgress is properly stored
// - All fields are correctly initialized on success
// - Errors from the initializer are properly propagated
func TestNewLatestPersistedSealedResult(t *testing.T) {
	resultID := unittest.IdentifierFixture()
	height := uint64(100)

	t.Run("successful initialization", func(t *testing.T) {
		mockInitializer := storagemock.NewConsumerProgressInitializer(t)
		mockCP := storagemock.NewConsumerProgress(t)

		mockInitializer.On("Initialize", height).Return(mockCP, nil)

		result, err := NewLatestPersistedSealedResult(resultID, height, mockInitializer)
		require.NoError(t, err)

		require.NotNil(t, result)
		assert.Equal(t, resultID, result.ResultID())
		assert.Equal(t, height, result.Height())
	})

	t.Run("initialization error", func(t *testing.T) {
		mockInitializer := storagemock.NewConsumerProgressInitializer(t)
		expectedErr := fmt.Errorf("initialization error: %w", storage.ErrNotFound)

		mockInitializer.On("Initialize", height).Return(nil, expectedErr)

		result, err := NewLatestPersistedSealedResult(resultID, height, mockInitializer)

		require.Error(t, err)
		require.Nil(t, result)
		assert.ErrorIs(t, err, storage.ErrNotFound)
	})
}

// TestLatestPersistedSealedResult_BatchSet tests the batch update functionality.
// It verifies that:
// - Updates are atomic - either all state is updated or none
// - The callback mechanism works correctly for both success and failure cases
// - State is not updated if BatchSetProcessedIndex fails
// - State is only updated after the batch callback indicates success
func TestLatestPersistedSealedResult_BatchSet(t *testing.T) {
	initialResultID := unittest.IdentifierFixture()
	initialHeight := uint64(100)

	t.Run("successful batch update", func(t *testing.T) {
		mockInitializer := storagemock.NewConsumerProgressInitializer(t)
		mockCP := storagemock.NewConsumerProgress(t)
		mockBatch := storagemock.NewReaderBatchWriter(t)

		mockInitializer.On("Initialize", initialHeight).Return(mockCP, nil)

		result, err := NewLatestPersistedSealedResult(initialResultID, initialHeight, mockInitializer)
		require.NoError(t, err)

		newResultID := unittest.IdentifierFixture()
		newHeight := uint64(200)

		var callbackCalled sync.WaitGroup
		callbackCalled.Add(1)

		mockCP.On("BatchSetProcessedIndex", newHeight, mockBatch).Return(nil)
		// Simulate successful batch commit by calling the callback with nil error
		mockBatch.On("AddCallback", mock.AnythingOfType("func(error)")).Run(func(args mock.Arguments) {
			callback := args.Get(0).(func(error))
			callback(nil)
			callbackCalled.Done()
		})

		err = result.BatchSet(newResultID, newHeight, mockBatch)
		require.NoError(t, err)

		// Wait for the callback to complete before checking state
		callbackCalled.Wait()

		assert.Equal(t, newResultID, result.ResultID())
		assert.Equal(t, newHeight, result.Height())
	})

	t.Run("batch update error during BatchSetProcessedIndex", func(t *testing.T) {
		mockInitializer := storagemock.NewConsumerProgressInitializer(t)
		mockCP := storagemock.NewConsumerProgress(t)
		mockBatch := storagemock.NewReaderBatchWriter(t)

		mockInitializer.On("Initialize", initialHeight).Return(mockCP, nil)

		result, err := NewLatestPersistedSealedResult(initialResultID, initialHeight, mockInitializer)
		require.NoError(t, err)

		newResultID := unittest.IdentifierFixture()
		newHeight := uint64(200)
		expectedErr := fmt.Errorf("batch update error: %w", storage.ErrNotFound)

		var callbackCalled sync.WaitGroup
		callbackCalled.Add(1)

		// The callback is registered before BatchSetProcessedIndex is called,
		// so we need to expect it and call it to release the lock
		mockBatch.On("AddCallback", mock.AnythingOfType("func(error)")).Run(func(args mock.Arguments) {
			callback := args.Get(0).(func(error))
			// Call the callback with the same error to ensure locks are released
			callback(storage.ErrNotFound)
			callbackCalled.Done()
		})

		// Simulate a failure in BatchSetProcessedIndex
		mockCP.On("BatchSetProcessedIndex", newHeight, mockBatch).Return(expectedErr)

		err = result.BatchSet(newResultID, newHeight, mockBatch)
		require.Error(t, err)
		assert.ErrorIs(t, err, storage.ErrNotFound)

		// Wait for the callback to complete to ensure locks are released
		callbackCalled.Wait()

		// Verify state remains unchanged on error
		assert.Equal(t, initialResultID, result.ResultID())
		assert.Equal(t, initialHeight, result.Height())
	})

	t.Run("batch update error during callback", func(t *testing.T) {
		mockInitializer := storagemock.NewConsumerProgressInitializer(t)
		mockCP := storagemock.NewConsumerProgress(t)
		mockBatch := storagemock.NewReaderBatchWriter(t)

		mockInitializer.On("Initialize", initialHeight).Return(mockCP, nil)

		result, err := NewLatestPersistedSealedResult(initialResultID, initialHeight, mockInitializer)
		require.NoError(t, err)

		newResultID := unittest.IdentifierFixture()
		newHeight := uint64(200)

		var callbackCalled sync.WaitGroup
		callbackCalled.Add(1)

		mockCP.On("BatchSetProcessedIndex", newHeight, mockBatch).Return(nil)
		// Simulate batch commit failure
		mockBatch.On("AddCallback", mock.AnythingOfType("func(error)")).Run(func(args mock.Arguments) {
			callback := args.Get(0).(func(error))
			callback(storage.ErrNotFound)
			callbackCalled.Done()
		})

		err = result.BatchSet(newResultID, newHeight, mockBatch)
		require.NoError(t, err)

		// Wait for the callback to complete before checking state
		callbackCalled.Wait()

		// Verify state remains unchanged when callback returns error
		assert.Equal(t, initialResultID, result.ResultID())
		assert.Equal(t, initialHeight, result.Height())
	})
}

// TestLatestPersistedSealedResult_ConcurrentAccess tests the thread safety of the implementation.
// It verifies that:
// - Multiple concurrent reads are safe
// - Concurrent reads and writes are properly synchronized
// - No data races occur under heavy concurrent load
// - The state remains consistent during concurrent operations
func TestLatestPersistedSealedResult_ConcurrentAccess(t *testing.T) {
	initialResultID := unittest.IdentifierFixture()
	initialHeight := uint64(100)
	mockInitializer := storagemock.NewConsumerProgressInitializer(t)
	mockCP := storagemock.NewConsumerProgress(t)

	mockInitializer.On("Initialize", initialHeight).Return(mockCP, nil)

	result, err := NewLatestPersistedSealedResult(initialResultID, initialHeight, mockInitializer)
	require.NoError(t, err)

	t.Run("concurrent reads", func(t *testing.T) {
		var wg sync.WaitGroup
		numGoroutines := 10

		// Launch multiple goroutines to read values concurrently
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				assert.Equal(t, initialResultID, result.ResultID())
				assert.Equal(t, initialHeight, result.Height())
			}()
		}

		wg.Wait()
	})

	t.Run("concurrent read/write", func(t *testing.T) {
		var wg sync.WaitGroup
		numGoroutines := 10

		mockBatch := storagemock.NewReaderBatchWriter(t)
		// Configure mock to accept any height and always succeed
		mockCP.On("BatchSetProcessedIndex", mock.Anything, mockBatch).Return(nil).Times(numGoroutines)

		var callbacksCompleted sync.WaitGroup
		callbacksCompleted.Add(numGoroutines)

		// Configure mock to simulate successful batch commits
		mockBatch.On("AddCallback", mock.AnythingOfType("func(error)")).Run(func(args mock.Arguments) {
			callback := args.Get(0).(func(error))
			callback(nil)
			callbacksCompleted.Done()
		}).Times(numGoroutines)

		// Launch pairs of goroutines - one writer and one reader
		for i := 0; i < numGoroutines; i++ {
			wg.Add(2)
			// Writer goroutine
			go func(i int) {
				defer wg.Done()
				newResultID := unittest.IdentifierFixture()
				newHeight := uint64(200 + i)
				err := result.BatchSet(newResultID, newHeight, mockBatch)
				require.NoError(t, err)
			}(i)
			// Reader goroutine
			go func() {
				defer wg.Done()
				_ = result.ResultID()
				_ = result.Height()
			}()
		}

		// Wait for all goroutines to complete their operations
		wg.Wait()
		// Wait for all callbacks to complete
		callbacksCompleted.Wait()
	})
}
