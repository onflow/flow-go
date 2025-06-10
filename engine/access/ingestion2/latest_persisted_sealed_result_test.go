package ingestion2

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
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
	height := uint64(100)

	t.Run("successful initialization", func(t *testing.T) {
		mockInitializer := storagemock.NewConsumerProgressInitializer(t)
		mockCP := storagemock.NewConsumerProgress(t)
		mockHeaders := storagemock.NewHeaders(t)
		mockResults := storagemock.NewExecutionResults(t)

		header := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(height))
		result := unittest.ExecutionResultFixture(func(result *flow.ExecutionResult) {
			result.BlockID = header.ID()
		})

		mockInitializer.On("Initialize", height).Return(mockCP, nil)
		mockCP.On("ProcessedIndex").Return(height, nil)
		mockHeaders.On("ByHeight", height).Return(header, nil)
		mockResults.On("ByBlockID", result.BlockID).Return(result, nil)

		latest, err := NewLatestPersistedSealedResult(height, mockInitializer, mockHeaders, mockResults)
		require.NoError(t, err)

		require.NotNil(t, latest)
		assert.Equal(t, result.ID(), latest.ResultID())
		assert.Equal(t, height, latest.Height())
	})

	t.Run("initialization error", func(t *testing.T) {
		mockInitializer := storagemock.NewConsumerProgressInitializer(t)
		expectedErr := fmt.Errorf("initialization error: %w", storage.ErrNotFound)

		mockInitializer.On("Initialize", height).Return(nil, expectedErr)

		latest, err := NewLatestPersistedSealedResult(height, mockInitializer, nil, nil)

		require.Error(t, err)
		require.Nil(t, latest)
		assert.ErrorIs(t, err, storage.ErrNotFound)
	})

	t.Run("processed index error", func(t *testing.T) {
		mockInitializer := storagemock.NewConsumerProgressInitializer(t)
		mockCP := storagemock.NewConsumerProgress(t)
		expectedErr := fmt.Errorf("processed index error: %w", storage.ErrNotFound)

		mockInitializer.On("Initialize", height).Return(mockCP, nil)
		mockCP.On("ProcessedIndex").Return(uint64(0), expectedErr)

		latest, err := NewLatestPersistedSealedResult(height, mockInitializer, nil, nil)

		require.Error(t, err)
		require.Nil(t, latest)
		assert.ErrorIs(t, err, storage.ErrNotFound)
	})

	t.Run("header lookup error", func(t *testing.T) {
		mockInitializer := storagemock.NewConsumerProgressInitializer(t)
		mockCP := storagemock.NewConsumerProgress(t)
		mockHeaders := storagemock.NewHeaders(t)
		expectedErr := fmt.Errorf("header lookup error: %w", storage.ErrNotFound)

		mockInitializer.On("Initialize", height).Return(mockCP, nil)
		mockCP.On("ProcessedIndex").Return(height, nil)
		mockHeaders.On("ByHeight", height).Return(nil, expectedErr)

		latest, err := NewLatestPersistedSealedResult(height, mockInitializer, mockHeaders, nil)

		require.Error(t, err)
		require.Nil(t, latest)
		assert.ErrorIs(t, err, storage.ErrNotFound)
	})

	t.Run("result lookup error", func(t *testing.T) {
		mockInitializer := storagemock.NewConsumerProgressInitializer(t)
		mockCP := storagemock.NewConsumerProgress(t)
		mockHeaders := storagemock.NewHeaders(t)
		mockResults := storagemock.NewExecutionResults(t)
		expectedErr := fmt.Errorf("result lookup error: %w", storage.ErrNotFound)

		header := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(height))

		mockInitializer.On("Initialize", height).Return(mockCP, nil)
		mockCP.On("ProcessedIndex").Return(height, nil)
		mockHeaders.On("ByHeight", height).Return(header, nil)
		mockResults.On("ByBlockID", header.ID()).Return(nil, expectedErr)

		latest, err := NewLatestPersistedSealedResult(height, mockInitializer, mockHeaders, mockResults)

		require.Error(t, err)
		require.Nil(t, latest)
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
	initialHeight := uint64(100)

	setupResult := func(t *testing.T) (*LatestPersistedSealedResult, *storagemock.ConsumerProgress) {
		mockInitializer := storagemock.NewConsumerProgressInitializer(t)
		mockCP := storagemock.NewConsumerProgress(t)
		mockHeaders := storagemock.NewHeaders(t)
		mockResults := storagemock.NewExecutionResults(t)

		header := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(initialHeight))
		result := unittest.ExecutionResultFixture(func(result *flow.ExecutionResult) {
			result.BlockID = header.ID()
		})

		mockInitializer.On("Initialize", initialHeight).Return(mockCP, nil)
		mockCP.On("ProcessedIndex").Return(initialHeight, nil)
		mockHeaders.On("ByHeight", initialHeight).Return(header, nil)
		mockResults.On("ByBlockID", result.BlockID).Return(result, nil)

		latest, err := NewLatestPersistedSealedResult(initialHeight, mockInitializer, mockHeaders, mockResults)
		require.NoError(t, err)
		return latest, mockCP
	}

	t.Run("successful batch update", func(t *testing.T) {
		latest, mockCP := setupResult(t)
		mockBatch := storagemock.NewReaderBatchWriter(t)

		newResultID := unittest.IdentifierFixture()
		newHeight := uint64(200)

		var callbackCalled sync.WaitGroup
		callbackCalled.Add(1)

		mockCP.On("BatchSetProcessedIndex", newHeight, mockBatch).Return(nil)
		mockBatch.On("AddCallback", mock.AnythingOfType("func(error)")).Run(func(args mock.Arguments) {
			callback := args.Get(0).(func(error))
			callback(nil)
			callbackCalled.Done()
		})

		err := latest.BatchSet(newResultID, newHeight, mockBatch)
		require.NoError(t, err)

		callbackCalled.Wait()

		assert.Equal(t, newResultID, latest.ResultID())
		assert.Equal(t, newHeight, latest.Height())
	})

	t.Run("batch update error during BatchSetProcessedIndex", func(t *testing.T) {
		latest, mockCP := setupResult(t)
		mockBatch := storagemock.NewReaderBatchWriter(t)

		newResultID := unittest.IdentifierFixture()
		newHeight := uint64(200)

		var callbackCalled sync.WaitGroup
		callbackCalled.Add(1)

		mockBatch.On("AddCallback", mock.AnythingOfType("func(error)")).Run(func(args mock.Arguments) {
			callback := args.Get(0).(func(error))
			callback(storage.ErrNotFound)
			callbackCalled.Done()
		})

		expectedErr := fmt.Errorf("could not set processed index: %w", storage.ErrNotFound)
		mockCP.On("BatchSetProcessedIndex", newHeight, mockBatch).Return(expectedErr)

		err := latest.BatchSet(newResultID, newHeight, mockBatch)

		require.Error(t, err)
		assert.ErrorIs(t, err, storage.ErrNotFound)

		callbackCalled.Wait()

		assert.NotEqual(t, newResultID, latest.ResultID())
		assert.Equal(t, initialHeight, latest.Height())
	})

	t.Run("batch update error during callback", func(t *testing.T) {
		latest, mockCP := setupResult(t)
		mockBatch := storagemock.NewReaderBatchWriter(t)

		newResultID := unittest.IdentifierFixture()
		newHeight := uint64(200)

		var callbackCalled sync.WaitGroup
		callbackCalled.Add(1)

		mockCP.On("BatchSetProcessedIndex", newHeight, mockBatch).Return(nil)
		mockBatch.On("AddCallback", mock.AnythingOfType("func(error)")).Run(func(args mock.Arguments) {
			callback := args.Get(0).(func(error))
			callback(storage.ErrNotFound)
			callbackCalled.Done()
		})

		err := latest.BatchSet(newResultID, newHeight, mockBatch)
		require.NoError(t, err)

		callbackCalled.Wait()

		assert.NotEqual(t, newResultID, latest.ResultID())
		assert.Equal(t, initialHeight, latest.Height())
	})
}

// TestLatestPersistedSealedResult_ConcurrentAccess tests the thread safety of the implementation.
// It verifies that:
// - Multiple concurrent reads are safe
// - Concurrent reads and writes are properly synchronized
// - No data races occur under heavy concurrent load
// - The state remains consistent during concurrent operations
func TestLatestPersistedSealedResult_ConcurrentAccess(t *testing.T) {
	initialHeight := uint64(100)
	mockInitializer := storagemock.NewConsumerProgressInitializer(t)
	mockCP := storagemock.NewConsumerProgress(t)
	mockHeaders := storagemock.NewHeaders(t)
	mockResults := storagemock.NewExecutionResults(t)

	header := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(initialHeight))
	result := unittest.ExecutionResultFixture(func(result *flow.ExecutionResult) {
		result.BlockID = header.ID()
	})

	mockInitializer.On("Initialize", initialHeight).Return(mockCP, nil)
	mockCP.On("ProcessedIndex").Return(initialHeight, nil)
	mockHeaders.On("ByHeight", initialHeight).Return(header, nil)
	mockResults.On("ByBlockID", result.BlockID).Return(result, nil)

	latest, err := NewLatestPersistedSealedResult(initialHeight, mockInitializer, mockHeaders, mockResults)
	require.NoError(t, err)

	t.Run("concurrent reads", func(t *testing.T) {
		var wg sync.WaitGroup
		numGoroutines := 1000

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				assert.Equal(t, result.ID(), latest.ResultID())
				assert.Equal(t, initialHeight, latest.Height())
			}()
		}

		wg.Wait()
	})

	t.Run("concurrent read/write", func(t *testing.T) {
		var wg sync.WaitGroup
		numGoroutines := 1000

		mockBatch := storagemock.NewReaderBatchWriter(t)
		mockCP.On("BatchSetProcessedIndex", mock.Anything, mockBatch).Return(nil).Times(numGoroutines)

		var callbacksCompleted sync.WaitGroup
		callbacksCompleted.Add(numGoroutines)

		mockBatch.On("AddCallback", mock.AnythingOfType("func(error)")).Run(func(args mock.Arguments) {
			callback := args.Get(0).(func(error))
			callback(nil)
			callbacksCompleted.Done()
		}).Times(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(2)
			go func(i int) {
				defer wg.Done()
				newResultID := unittest.IdentifierFixture()
				newHeight := uint64(200 + i)
				err := latest.BatchSet(newResultID, newHeight, mockBatch)
				require.NoError(t, err)
			}(i)
			go func() {
				defer wg.Done()
				_ = latest.ResultID()
				_ = latest.Height()
			}()
		}

		wg.Wait()
		callbacksCompleted.Wait()
	})
}
