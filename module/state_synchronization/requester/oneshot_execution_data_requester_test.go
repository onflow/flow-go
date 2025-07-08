package requester

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	edmock "github.com/onflow/flow-go/module/executiondatasync/execution_data/mock"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

type OneshotExecutionDataRequesterSuite struct {
	suite.Suite
}

func TestRawExecutionDataRequesterSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(OneshotExecutionDataRequesterSuite))
}

func (suite *OneshotExecutionDataRequesterSuite) TestRequestExecutionData() {
	logger := unittest.Logger()
	metricsCollector := metrics.NewNoopCollector()
	config := OneshotExecutionDataConfig{
		FetchTimeout:    DefaultFetchTimeout,
		MaxFetchTimeout: DefaultMaxFetchTimeout,
		RetryDelay:      DefaultRetryDelay,
		MaxRetryDelay:   DefaultMaxRetryDelay,
	}

	suite.Run("Happy path. Raw setup", func() {
		block := unittest.BlockFixture()
		result := unittest.ExecutionResultFixture(unittest.WithBlock(block))
		blockEd := unittest.BlockExecutionDataFixture(unittest.WithBlockExecutionDataBlockID(block.ID()))

		execDataDownloader := edmock.NewDownloader(suite.T())
		execDataDownloader.
			On("Get", mock.Anything, mock.AnythingOfType("flow.Identifier")).
			Return(blockEd, nil).
			Once()

		requester, err := NewOneshotExecutionDataRequester(logger, metricsCollector, execDataDownloader, result, block.ToHeader(), config)
		require.NoError(suite.T(), err)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		execData, err := requester.RequestExecutionData(ctx)
		require.NoError(suite.T(), err)
		require.Equal(suite.T(), result.BlockID, execData.BlockID)
	})

	suite.Run("Happy path. Full storages setup", func() {
		dataStore := dssync.MutexWrap(datastore.NewMapDatastore())
		blobstore := blobs.NewBlobstore(dataStore)
		testData := generateTestData(suite.T(), blobstore, 5, map[uint64]testExecutionDataCallback{})
		execDataDownloader := MockDownloader(testData.executionDataEntries)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Test each sealed block
		for blockID, expectedED := range testData.executionDataByID {
			block := testData.blocksByID[blockID]
			result := testData.resultsByBlockID[blockID]

			requester, err := NewOneshotExecutionDataRequester(
				logger,
				metricsCollector,
				execDataDownloader,
				result,
				block.ToHeader(),
				config,
			)
			require.NoError(suite.T(), err)
			require.NotNil(suite.T(), requester)

			execData, err := requester.RequestExecutionData(ctx)
			require.NoError(suite.T(), err)
			require.Equal(suite.T(), execData.BlockID, result.BlockID)
			require.Equal(suite.T(), expectedED.BlockID, execData.BlockID)
			require.Equal(suite.T(), expectedED.ChunkExecutionDatas, execData.ChunkExecutionDatas)
		}
	})
}

func (suite *OneshotExecutionDataRequesterSuite) TestRequestExecution_ERCacheReturnsError() {
	logger := unittest.Logger()
	metricsCollector := metrics.NewNoopCollector()
	config := OneshotExecutionDataConfig{
		FetchTimeout:    DefaultFetchTimeout,
		MaxFetchTimeout: DefaultMaxFetchTimeout,
		RetryDelay:      DefaultRetryDelay,
		MaxRetryDelay:   DefaultMaxRetryDelay,
	}

	// Create a block and execution result
	block := unittest.BlockFixture()
	executionResult := unittest.ExecutionResultFixture(unittest.WithBlock(block))

	suite.Run("blob not found error", func() {
		// Mock downloader to return blob not found error first, then success
		execDataDownloader := edmock.NewDownloader(suite.T())
		expectedError := &execution_data.BlobNotFoundError{}
		execDataDownloader.
			On("Get", mock.Anything, executionResult.ExecutionDataID).
			Return(nil, expectedError).
			Once()

		// Eventually return execution data
		blockEd := unittest.BlockExecutionDataFixture(unittest.WithBlockExecutionDataBlockID(block.ID()))
		execDataDownloader.
			On("Get", mock.Anything, executionResult.ExecutionDataID).
			Return(blockEd, nil).
			Once()

		requester, err := NewOneshotExecutionDataRequester(logger, metricsCollector, execDataDownloader, executionResult, block.ToHeader(), config)
		require.NoError(suite.T(), err)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		execData, err := requester.RequestExecutionData(ctx)
		require.NoError(suite.T(), err)
		require.NotNil(suite.T(), execData)
		require.Equal(suite.T(), block.ID(), execData.BlockID)
	})

	suite.Run("deadline exceeded error", func() {
		// Mock downloader to return not found error
		execDataDownloader := edmock.NewDownloader(suite.T())
		expectedError := context.DeadlineExceeded
		execDataDownloader.
			On("Get", mock.Anything, executionResult.ExecutionDataID).
			Return(nil, expectedError).
			Maybe()

		requester, err := NewOneshotExecutionDataRequester(logger, metricsCollector, execDataDownloader, executionResult, block.ToHeader(), config)
		require.NoError(suite.T(), err)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Should retry until timeout
		execData, err := requester.RequestExecutionData(ctx)
		require.ErrorIs(suite.T(), err, expectedError)
		require.Nil(suite.T(), execData)
	})

	suite.Run("malformed data error", func() {
		// Mock downloader to return malformed data error
		execDataDownloader := edmock.NewDownloader(suite.T())
		expectedError := execution_data.NewMalformedDataError(fmt.Errorf("malformed data"))
		execDataDownloader.
			On("Get", mock.Anything, executionResult.ExecutionDataID).
			Return(nil, expectedError).
			Once()

		requester, err := NewOneshotExecutionDataRequester(logger, metricsCollector, execDataDownloader, executionResult, block.ToHeader(), config)
		require.NoError(suite.T(), err)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		execData, err := requester.RequestExecutionData(ctx)
		require.ErrorIs(suite.T(), err, expectedError)
		require.Nil(suite.T(), execData)
	})

	suite.Run("blob size limit exceed error", func() {
		// Mock downloader to return blob size limit exceeded error
		execDataDownloader := edmock.NewDownloader(suite.T())
		expectedError := &execution_data.BlobSizeLimitExceededError{}
		execDataDownloader.
			On("Get", mock.Anything, executionResult.ExecutionDataID).
			Return(nil, expectedError).
			Once()

		requester, err := NewOneshotExecutionDataRequester(logger, metricsCollector, execDataDownloader, executionResult, block.ToHeader(), config)
		require.NoError(suite.T(), err)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		execData, err := requester.RequestExecutionData(ctx)
		require.ErrorIs(suite.T(), err, expectedError)
		require.Nil(suite.T(), execData)
	})

	suite.Run("context canceled error", func() {
		// Return context.DeadlineExceeded to trigger retry logic
		execDataDownloader := edmock.NewDownloader(suite.T())
		execDataDownloader.
			On("Get", mock.Anything, executionResult.ExecutionDataID).
			Return(nil, context.DeadlineExceeded).
			Once()

		// Eventually return context.Canceled to stop downloader's retry logic
		expectedError := context.Canceled
		execDataDownloader.
			On("Get", mock.Anything, executionResult.ExecutionDataID).
			Return(nil, expectedError).
			Once()

		requester, err := NewOneshotExecutionDataRequester(logger, metricsCollector, execDataDownloader, executionResult, block.ToHeader(), config)
		require.NoError(suite.T(), err)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		execData, err := requester.RequestExecutionData(ctx)
		require.ErrorIs(suite.T(), err, expectedError)
		require.Nil(suite.T(), execData)
	})
}
