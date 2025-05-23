package requester

import (
	"context"
	"fmt"
	"testing"

	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/storage"

	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/module/blobs"
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
		seal := unittest.Seal.Fixture()
		seals.
			On("FinalizedSealForBlock", mock.AnythingOfType("flow.Identifier")).
			Return(seal, nil).
			Once()

		results := storagemock.NewExecutionResults(suite.T())
		result := unittest.ExecutionResultFixture()
		results.
			On("ByID", mock.AnythingOfType("flow.Identifier")).
			Return(result, nil).
			Once()

		blockEd := unittest.BlockExecutionDataFixture()
		heroCache := herocache.NewBlockExecutionData(subscription.DefaultCacheSize, logger, metricsCollector)

		downloader := edmock.NewDownloader(suite.T())
		downloader.
			On("Get", mock.Anything, mock.AnythingOfType("flow.Identifier")).
			Return(blockEd, nil).
			Once()

		edCache := cache.NewExecutionDataCache(downloader, headers, seals, results, heroCache)
		requester := NewOneshotExecutionDataRequester(logger, metricsCollector, edCache, config)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := requester.RequestExecutionData(ctx, blockEd.BlockID, 0)
		require.NoError(suite.T(), err)

		// Requester doesn't return downloaded execution data. It puts them into the internal cache.
		// So, here we check if we successfully put the execution data into the cache.
		require.True(suite.T(), heroCache.Has(blockEd.BlockID))
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
		requester := NewOneshotExecutionDataRequester(logger, metricsCollector, edCache, config)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		for blockID := range testData.executionDataIDByBlockID {
			// height is used only for logging purposes
			err := requester.RequestExecutionData(ctx, blockID, 0)
			require.NoError(suite.T(), err)

			// Requester doesn't return downloaded execution data. It puts them into the internal cache.
			// So, here we check if we successfully put the execution data into the cache.
			require.True(suite.T(), heroCache.Has(blockID))
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
	seal := unittest.Seal.Fixture()
	seals.
		On("FinalizedSealForBlock", mock.AnythingOfType("flow.Identifier")).
		Return(seal, nil)

	blockEd := unittest.BlockExecutionDataFixture()
	downloader := edmock.NewDownloader(suite.T())
	downloader.
		On("Get", mock.Anything, mock.AnythingOfType("flow.Identifier")).
		Return(blockEd, nil)

	suite.Run("blob not found error", func() {
		// first time return blob not found error to test retry mechanism
		results := storagemock.NewExecutionResults(suite.T())
		expectedError := execution_data.BlobNotFoundError{}
		results.
			On("ByID", mock.AnythingOfType("flow.Identifier")).
			Return(nil, &expectedError).
			Once()

		// eventually return an execution result
		expectedResult := unittest.ExecutionResultFixture()
		results.
			On("ByID", mock.AnythingOfType("flow.Identifier")).
			Return(expectedResult, nil).
			Once()

		heroCache := herocache.NewBlockExecutionData(subscription.DefaultCacheSize, logger, metricsCollector)
		edCache := cache.NewExecutionDataCache(downloader, headers, seals, results, heroCache)
		requester := NewOneshotExecutionDataRequester(logger, metricsCollector, edCache, config)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := requester.RequestExecutionData(ctx, blockEd.BlockID, 0)
		require.NoError(suite.T(), err)

		// Requester doesn't return downloaded execution data. It puts them into the internal cache.
		// Make sure execution data got into cache eventually.
		require.True(suite.T(), heroCache.Has(blockEd.BlockID))
	})

	suite.Run("execution data not found in storage", func() {
		results := storagemock.NewExecutionResults(suite.T())
		expectedError := storage.ErrNotFound
		results.
			On("ByID", mock.AnythingOfType("flow.Identifier")).
			Return(nil, expectedError).
			Once()

		heroCache := herocache.NewBlockExecutionData(subscription.DefaultCacheSize, logger, metricsCollector)
		edCache := cache.NewExecutionDataCache(downloader, headers, seals, results, heroCache)
		requester := NewOneshotExecutionDataRequester(logger, metricsCollector, edCache, config)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := requester.RequestExecutionData(ctx, blockEd.BlockID, 0)
		require.ErrorIs(suite.T(), err, expectedError)

		// Requester doesn't return downloaded execution data. It puts them into the internal cache.
		require.False(suite.T(), heroCache.Has(blockEd.BlockID))
	})

	suite.Run("malformed data error", func() {
		results := storagemock.NewExecutionResults(suite.T())
		expectedError := execution_data.NewMalformedDataError(fmt.Errorf("malformed data"))
		results.
			On("ByID", mock.AnythingOfType("flow.Identifier")).
			Return(nil, expectedError).
			Once()

		heroCache := herocache.NewBlockExecutionData(subscription.DefaultCacheSize, logger, metricsCollector)
		edCache := cache.NewExecutionDataCache(downloader, headers, seals, results, heroCache)
		requester := NewOneshotExecutionDataRequester(logger, metricsCollector, edCache, config)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := requester.RequestExecutionData(ctx, blockEd.BlockID, 0)
		require.ErrorIs(suite.T(), err, expectedError)

		// Requester doesn't return downloaded execution data. It puts them into the internal cache.
		require.False(suite.T(), heroCache.Has(blockEd.BlockID))
	})

	suite.Run("blob size limit exceed error", func() {
		results := storagemock.NewExecutionResults(suite.T())
		expectedError := execution_data.BlobSizeLimitExceededError{}
		results.
			On("ByID", mock.AnythingOfType("flow.Identifier")).
			Return(nil, &expectedError).
			Once()

		heroCache := herocache.NewBlockExecutionData(subscription.DefaultCacheSize, logger, metricsCollector)
		edCache := cache.NewExecutionDataCache(downloader, headers, seals, results, heroCache)
		requester := NewOneshotExecutionDataRequester(logger, metricsCollector, edCache, config)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := requester.RequestExecutionData(ctx, blockEd.BlockID, 0)
		require.ErrorIs(suite.T(), err, &expectedError)

		// Requester doesn't return downloaded execution data. It puts them into the internal cache.
		require.False(suite.T(), heroCache.Has(blockEd.BlockID))
	})
}
