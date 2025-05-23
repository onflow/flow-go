package requester

import (
	"context"
	"testing"

	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data/cache"
	edmock "github.com/onflow/flow-go/module/executiondatasync/execution_data/mock"
	"github.com/onflow/flow-go/module/irrecoverable"
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

func (suite *OneshotExecutionDataRequesterSuite) TestRequester_RawRequestExecutionData() {
	logger := unittest.Logger()
	metricsCollector := metrics.NewNoopCollector()
	config := OneshotExecutionDataConfig{
		FetchTimeout:    DefaultFetchTimeout,
		MaxFetchTimeout: DefaultMaxFetchTimeout,
		RetryDelay:      DefaultRetryDelay,
		MaxRetryDelay:   DefaultMaxRetryDelay,
	}

	headers := new(storagemock.Headers)
	seals := new(storagemock.Seals)
	seal := unittest.Seal.Fixture()
	seals.
		On("FinalizedSealForBlock", mock.AnythingOfType("flow.Identifier")).
		Return(seal, nil).
		Once()

	results := new(storagemock.ExecutionResults)
	result := unittest.ExecutionResultFixture()
	results.
		On("ByID", mock.AnythingOfType("flow.Identifier")).
		Return(result, nil).
		Once()

	blockEd := unittest.BlockExecutionDataFixture()
	heroCache := herocache.NewBlockExecutionData(subscription.DefaultCacheSize, logger, metricsCollector)

	downloader := new(edmock.Downloader)
	downloader.
		On("Get", mock.Anything, mock.AnythingOfType("flow.Identifier")).
		Return(blockEd, nil).
		Once()

	edCache := cache.NewExecutionDataCache(downloader, headers, seals, results, heroCache)
	requester := NewOneshotExecutionDataRequester(logger, metricsCollector, edCache, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	signalerCtx := irrecoverable.NewMockSignalerContext(suite.T(), ctx)

	err := requester.RequestExecutionData(signalerCtx, blockEd.BlockID, 0)
	require.NoError(suite.T(), err)

	// Requester doesn't return downloaded execution data. It puts them into the internal cache.
	// So, here we check if we successfully put the execution data into the cache.
	require.True(suite.T(), heroCache.Has(blockEd.BlockID))
}

func (suite *OneshotExecutionDataRequesterSuite) TestRequester_RequestExecutionData() {
	logger := unittest.Logger()
	metricsCollector := metrics.NewNoopCollector()
	config := OneshotExecutionDataConfig{
		FetchTimeout:    DefaultFetchTimeout,
		MaxFetchTimeout: DefaultMaxFetchTimeout,
		RetryDelay:      DefaultRetryDelay,
		MaxRetryDelay:   DefaultMaxRetryDelay,
	}

	datastore := dssync.MutexWrap(datastore.NewMapDatastore())
	blobstore := blobs.NewBlobstore(datastore)
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
	signalerCtx := irrecoverable.NewMockSignalerContext(suite.T(), ctx)

	for blockID := range testData.executionDataIDByBlockID {
		// height is used only for logging purposes
		err := requester.RequestExecutionData(signalerCtx, blockID, 0)
		require.NoError(suite.T(), err)

		// Requester doesn't return downloaded execution data. It puts them into the internal cache.
		// So, here we check if we successfully put the execution data into the cache.
		require.True(suite.T(), heroCache.Has(blockID))
	}
}
