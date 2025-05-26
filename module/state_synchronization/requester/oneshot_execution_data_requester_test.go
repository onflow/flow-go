package requester

import (
	"context"
	"fmt"
	"testing"

	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data/cache"
	edmock "github.com/onflow/flow-go/module/executiondatasync/execution_data/mock"
	"github.com/onflow/flow-go/module/mempool/herocache"
	"github.com/onflow/flow-go/module/metrics"
	synctest "github.com/onflow/flow-go/module/state_synchronization/requester/unittest"
	storagemock "github.com/onflow/flow-go/storage/mock"
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
		headers := storagemock.NewHeaders(suite.T())
		seals := storagemock.NewSeals(suite.T())

		results := storagemock.NewExecutionResults(suite.T())
		block := unittest.BlockFixture()
		result := unittest.ExecutionResultFixture(unittest.WithBlock(&block))

		blockEd := unittest.BlockExecutionDataFixture(unittest.WithBlockExecutionDataBlockID(block.ID()))

		heroCache := herocache.NewBlockExecutionData(subscription.DefaultCacheSize, logger, metricsCollector)

		downloader := edmock.NewDownloader(suite.T())
		downloader.
			On("Get", mock.Anything, mock.AnythingOfType("flow.Identifier")).
			Return(blockEd, nil).
			Once()

		edCache := cache.NewExecutionDataCache(downloader, headers, seals, results, heroCache)
		requester, err := NewOneshotExecutionDataRequester(logger, metricsCollector, edCache, result, block.Header, config)
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

		headers := synctest.MockBlockHeaderStorage(
			synctest.WithByID(testData.blocksByID),
			synctest.WithByHeight(testData.blocksByHeight),
			synctest.WithBlockIDByHeight(testData.blocksByHeight),
		)
		results := synctest.MockResultsStorage(
			synctest.WithResultByID(testData.resultsByID),
		)
		seals := synctest.MockSealsStorage(
			synctest.WithSealsByBlockID(testData.sealsByBlockID),
		)

		downloader := MockDownloader(testData.executionDataEntries)
		heroCache := herocache.NewBlockExecutionData(subscription.DefaultCacheSize, logger, metricsCollector)
		edCache := cache.NewExecutionDataCache(downloader, headers, seals, results, heroCache)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Test each sealed block
		for blockID, expectedED := range testData.executionDataByID {
			block := testData.blocksByID[blockID]
			result := testData.resultsByBlockID[blockID]

			requester, err := NewOneshotExecutionDataRequester(
				logger,
				metricsCollector,
				edCache,
				result,
				block.Header,
				config,
			)
			require.NoError(suite.T(), err)
			require.NotNil(suite.T(), requester)

			execData, err := requester.RequestExecutionData(ctx)
			require.NoError(suite.T(), err)

			require.Equal(suite.T(), execData.BlockID, result.BlockID)
			require.Equal(suite.T(), expectedED.BlockID, execData.BlockExecutionData.BlockID)
			require.Equal(suite.T(), expectedED.ChunkExecutionDatas, execData.BlockExecutionData.ChunkExecutionDatas)
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

	headers := storagemock.NewHeaders(suite.T())
	seals := storagemock.NewSeals(suite.T())

	// Create a block and execution result
	block := unittest.BlockFixture()
	executionResult := unittest.ExecutionResultFixture(unittest.WithBlock(&block))
	results := storagemock.NewExecutionResults(suite.T())

	suite.Run("blob not found error", func() {
		// Mock downloader to return blob not found error first, then success
		downloader := edmock.NewDownloader(suite.T())
		expectedError := &execution_data.BlobNotFoundError{}
		downloader.
			On("Get", mock.Anything, executionResult.ExecutionDataID).
			Return(nil, expectedError).
			Once()

		// eventually return execution data
		blockEd := unittest.BlockExecutionDataFixture(unittest.WithBlockExecutionDataBlockID(block.ID()))
		downloader.
			On("Get", mock.Anything, executionResult.ExecutionDataID).
			Return(blockEd, nil).
			Once()

		heroCache := herocache.NewBlockExecutionData(subscription.DefaultCacheSize, logger, metricsCollector)
		edCache := cache.NewExecutionDataCache(downloader, headers, seals, results, heroCache)
		requester, err := NewOneshotExecutionDataRequester(logger, metricsCollector, edCache, executionResult, block.Header, config)
		require.NoError(suite.T(), err)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		execData, err := requester.RequestExecutionData(ctx)
		require.NoError(suite.T(), err)
		require.NotNil(suite.T(), execData)
		require.Equal(suite.T(), block.ID(), execData.BlockID)
	})

	suite.Run("execution data not found in storage", func() {
		// Mock downloader to return not found error
		downloader := edmock.NewDownloader(suite.T())
		expectedError := &execution_data.BlobNotFoundError{}
		downloader.
			On("Get", mock.Anything, executionResult.ExecutionDataID).
			Return(nil, expectedError).
			Maybe()

		heroCache := herocache.NewBlockExecutionData(subscription.DefaultCacheSize, logger, metricsCollector)
		edCache := cache.NewExecutionDataCache(downloader, headers, seals, results, heroCache)
		requester, err := NewOneshotExecutionDataRequester(logger, metricsCollector, edCache, executionResult, block.Header, config)
		require.NoError(suite.T(), err)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		execData, err := requester.RequestExecutionData(ctx)
		require.ErrorIs(suite.T(), err, expectedError)
		require.Nil(suite.T(), execData)
	})

	suite.Run("malformed data error", func() {
		// Mock downloader to return malformed data error
		downloader := edmock.NewDownloader(suite.T())
		expectedError := execution_data.NewMalformedDataError(fmt.Errorf("malformed data"))
		downloader.
			On("Get", mock.Anything, executionResult.ExecutionDataID).
			Return(nil, expectedError).
			Once()

		heroCache := herocache.NewBlockExecutionData(subscription.DefaultCacheSize, logger, metricsCollector)
		edCache := cache.NewExecutionDataCache(downloader, headers, seals, results, heroCache)
		requester, err := NewOneshotExecutionDataRequester(logger, metricsCollector, edCache, executionResult, block.Header, config)
		require.NoError(suite.T(), err)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		execData, err := requester.RequestExecutionData(ctx)
		require.ErrorIs(suite.T(), err, expectedError)
		require.Nil(suite.T(), execData)
	})

	suite.Run("blob size limit exceed error", func() {
		// Mock downloader to return blob size limit exceeded error
		downloader := edmock.NewDownloader(suite.T())
		expectedError := &execution_data.BlobSizeLimitExceededError{}
		downloader.
			On("Get", mock.Anything, executionResult.ExecutionDataID).
			Return(nil, expectedError).
			Once()

		heroCache := herocache.NewBlockExecutionData(subscription.DefaultCacheSize, logger, metricsCollector)
		edCache := cache.NewExecutionDataCache(downloader, headers, seals, results, heroCache)
		requester, err := NewOneshotExecutionDataRequester(logger, metricsCollector, edCache, executionResult, block.Header, config)
		require.NoError(suite.T(), err)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		execData, err := requester.RequestExecutionData(ctx)
		require.ErrorIs(suite.T(), err, expectedError)
		require.Nil(suite.T(), execData)
	})
}
