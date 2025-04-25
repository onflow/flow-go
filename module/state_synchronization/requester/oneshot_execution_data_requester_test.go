package requester

import (
	"context"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data/cache"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool/herocache"
	"github.com/onflow/flow-go/module/metrics"
	synctest "github.com/onflow/flow-go/module/state_synchronization/requester/unittest"
	"github.com/onflow/flow-go/utils/unittest"
)

type OneshotExecutionDataRequesterSuite struct {
	suite.Suite
}

func TestRawExecutionDataRequesterSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(OneshotExecutionDataRequesterSuite))
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

	suite.T().Run("requester downloads all execution data", func(t *testing.T) {
		for blockID, _ := range testData.executionDataIDByBlockID {
			// height is used only for logging purposes
			err := requester.RequestExecutionData(signalerCtx, blockID, 0)
			require.NoError(t, err)
		}
	})
}

func generateTestData(t *testing.T, blobstore blobs.Blobstore, blockCount int, specialHeightFuncs map[uint64]testExecutionDataCallback) *fetchTestRun {
	edsEntries := map[flow.Identifier]*testExecutionDataServiceEntry{}
	blocksByHeight := map[uint64]*flow.Block{}
	blocksByID := map[flow.Identifier]*flow.Block{}
	resultsByID := map[flow.Identifier]*flow.ExecutionResult{}
	resultsByBlockID := map[flow.Identifier]*flow.ExecutionResult{}
	sealsByBlockID := map[flow.Identifier]*flow.Seal{}
	executionDataByID := map[flow.Identifier]*execution_data.BlockExecutionData{}
	executionDataIDByBlockID := map[flow.Identifier]flow.Identifier{}

	sealedCount := blockCount - 4 // seals for blocks 1-96
	firstSeal := blockCount - sealedCount

	// genesis is block 0, we start syncing from block 1
	startHeight := uint64(1)
	endHeight := uint64(blockCount) - 1

	// instantiate ExecutionDataService to generate correct CIDs
	eds := execution_data.NewExecutionDataStore(blobstore, execution_data.DefaultSerializer)

	var previousBlock *flow.Block
	var previousResult *flow.ExecutionResult
	for i := 0; i < blockCount; i++ {
		var seals []*flow.Header

		if i >= firstSeal {
			sealedBlock := blocksByHeight[uint64(i-firstSeal+1)]
			seals = []*flow.Header{
				sealedBlock.Header, // block 0 doesn't get sealed (it's pre-sealed in the genesis state)
			}

			sealsByBlockID[sealedBlock.ID()] = unittest.Seal.Fixture(
				unittest.Seal.WithBlockID(sealedBlock.ID()),
				unittest.Seal.WithResult(resultsByBlockID[sealedBlock.ID()]),
			)

			t.Logf("block %d has seals for %d", i, seals[0].Height)
		}

		height := uint64(i)
		block := buildBlock(height, previousBlock, seals)

		ed := unittest.BlockExecutionDataFixture(unittest.WithBlockExecutionDataBlockID(block.ID()))

		cid, err := eds.Add(context.Background(), ed)
		require.NoError(t, err)

		result := buildResult(block, cid, previousResult)

		blocksByHeight[height] = block
		blocksByID[block.ID()] = block
		resultsByBlockID[block.ID()] = result
		resultsByID[result.ID()] = result

		// ignore all the data we don't need to verify the test
		if i > 0 && i <= sealedCount {
			executionDataByID[block.ID()] = ed
			edsEntries[cid] = &testExecutionDataServiceEntry{ExecutionData: ed}
			if fn, has := specialHeightFuncs[height]; has {
				edsEntries[cid].fn = fn
			}

			executionDataIDByBlockID[block.ID()] = cid
		}

		previousBlock = block
		previousResult = result
	}

	return &fetchTestRun{
		sealedCount:              sealedCount,
		startHeight:              startHeight,
		endHeight:                endHeight,
		blocksByHeight:           blocksByHeight,
		blocksByID:               blocksByID,
		resultsByBlockID:         resultsByBlockID,
		resultsByID:              resultsByID,
		sealsByBlockID:           sealsByBlockID,
		executionDataByID:        executionDataByID,
		executionDataEntries:     edsEntries,
		executionDataIDByBlockID: executionDataIDByBlockID,
		waitTimeout:              time.Second * 5,

		maxSearchAhead: DefaultMaxSearchAhead,
		fetchTimeout:   DefaultFetchTimeout,
		retryDelay:     1 * time.Millisecond,
		maxRetryDelay:  15 * time.Millisecond,
	}
}
