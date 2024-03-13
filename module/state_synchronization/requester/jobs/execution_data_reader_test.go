package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data/cache"
	exedatamock "github.com/onflow/flow-go/module/executiondatasync/execution_data/mock"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool/herocache"
	"github.com/onflow/flow-go/module/metrics"
	synctest "github.com/onflow/flow-go/module/state_synchronization/requester/unittest"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type ExecutionDataReaderSuite struct {
	suite.Suite

	reader       *ExecutionDataReader
	downloader   *exedatamock.Downloader
	headers      *storagemock.Headers
	results      *storagemock.ExecutionResults
	seals        *storagemock.Seals
	fetchTimeout time.Duration

	executionDataID flow.Identifier
	executionData   *execution_data.BlockExecutionData
	block           *flow.Block
	blocksByHeight  map[uint64]*flow.Block

	highestAvailableHeight func() uint64
}

func TestExecutionDataReaderSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(ExecutionDataReaderSuite))
}

func (suite *ExecutionDataReaderSuite) SetupTest() {
	suite.fetchTimeout = time.Second
	suite.executionDataID = unittest.IdentifierFixture()

	parent := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(1))
	suite.block = unittest.BlockWithParentFixture(parent)
	suite.blocksByHeight = map[uint64]*flow.Block{
		suite.block.Header.Height: suite.block,
	}

	suite.executionData = unittest.BlockExecutionDataFixture(unittest.WithBlockExecutionDataBlockID(suite.block.ID()))

	suite.highestAvailableHeight = func() uint64 { return suite.block.Header.Height + 1 }

	suite.reset()
}

func (suite *ExecutionDataReaderSuite) reset() {
	result := unittest.ExecutionResultFixture(
		unittest.WithBlock(suite.block),
		unittest.WithExecutionDataID(suite.executionDataID),
	)

	seal := unittest.Seal.Fixture(
		unittest.Seal.WithBlockID(suite.block.ID()),
		unittest.Seal.WithResult(result),
	)

	suite.headers = synctest.MockBlockHeaderStorage(
		synctest.WithByHeight(suite.blocksByHeight),
		synctest.WithBlockIDByHeight(suite.blocksByHeight),
	)
	suite.results = synctest.MockResultsStorage(
		synctest.WithResultByID(map[flow.Identifier]*flow.ExecutionResult{
			result.ID(): result,
		}),
	)
	suite.seals = synctest.MockSealsStorage(
		synctest.WithSealsByBlockID(map[flow.Identifier]*flow.Seal{
			suite.block.ID(): seal,
		}),
	)

	suite.downloader = new(exedatamock.Downloader)
	var executionDataCacheSize uint32 = 100 // Use local value to avoid cycle dependency on subscription package

	heroCache := herocache.NewBlockExecutionData(executionDataCacheSize, unittest.Logger(), metrics.NewNoopCollector())
	cache := cache.NewExecutionDataCache(suite.downloader, suite.headers, suite.seals, suite.results, heroCache)

	suite.reader = NewExecutionDataReader(
		cache,
		suite.fetchTimeout,
		func() (uint64, error) {
			return suite.highestAvailableHeight(), nil
		},
	)
}

func (suite *ExecutionDataReaderSuite) TestAtIndex() {
	setExecutionDataGet := func(executionData *execution_data.BlockExecutionData, err error) {
		suite.downloader.On("Get", mock.Anything, suite.executionDataID).Return(
			func(ctx context.Context, id flow.Identifier) *execution_data.BlockExecutionData {
				return executionData
			},
			func(ctx context.Context, id flow.Identifier) error {
				return err
			},
		)
	}

	suite.Run("returns not found when not initialized", func() {
		// runTest not called, so context is never added
		job, err := suite.reader.AtIndex(1)
		assert.Nil(suite.T(), job, "job should be nil")
		assert.Error(suite.T(), err, "error should be returned")
	})

	suite.Run("returns not found when index out of range", func() {
		suite.reset()
		suite.runTest(func() {
			job, err := suite.reader.AtIndex(suite.highestAvailableHeight() + 1)
			assert.Nil(suite.T(), job, "job should be nil")
			assert.Equal(suite.T(), storage.ErrNotFound, err, "expected not found error")
		})
	})

	suite.Run("returns successfully", func() {
		suite.reset()
		suite.runTest(func() {
			ed := unittest.BlockExecutionDataFixture()
			setExecutionDataGet(ed, nil)

			edEntity := execution_data.NewBlockExecutionDataEntity(suite.executionDataID, ed)

			job, err := suite.reader.AtIndex(suite.block.Header.Height)
			require.NoError(suite.T(), err)

			entry, err := JobToBlockEntry(job)
			assert.NoError(suite.T(), err)

			assert.Equal(suite.T(), edEntity, entry.ExecutionData)
		})
	})

	suite.Run("returns error from ExecutionDataService Get", func() {
		suite.reset()
		suite.runTest(func() {
			// return an error while getting the execution data
			expectedErr := errors.New("expected error: get failed")
			setExecutionDataGet(nil, expectedErr)

			job, err := suite.reader.AtIndex(suite.block.Header.Height)
			assert.Nil(suite.T(), job, "job should be nil")
			assert.ErrorIs(suite.T(), err, expectedErr)
		})
	})

	suite.Run("returns error getting header", func() {
		suite.reset()
		suite.runTest(func() {
			// search for an index that doesn't have a header in storage
			job, err := suite.reader.AtIndex(suite.block.Header.Height + 1)
			assert.Nil(suite.T(), job, "job should be nil")
			assert.ErrorIs(suite.T(), err, storage.ErrNotFound)
		})
	})

	suite.Run("returns error getting execution result", func() {
		suite.reset()
		suite.runTest(func() {
			// add a new block without an execution result
			newBlock := unittest.BlockWithParentFixture(suite.block.Header)
			suite.blocksByHeight[newBlock.Header.Height] = newBlock

			job, err := suite.reader.AtIndex(newBlock.Header.Height)
			assert.Nil(suite.T(), job, "job should be nil")
			assert.ErrorIs(suite.T(), err, storage.ErrNotFound)
		})
	})
}

func (suite *ExecutionDataReaderSuite) TestHead() {
	suite.runTest(func() {
		expectedIndex := uint64(15)
		suite.highestAvailableHeight = func() uint64 {
			return expectedIndex
		}
		index, err := suite.reader.Head()
		assert.NoError(suite.T(), err)
		assert.Equal(suite.T(), expectedIndex, index)
	})
}

func (suite *ExecutionDataReaderSuite) runTest(fn func()) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signalerCtx := irrecoverable.NewMockSignalerContext(suite.T(), ctx)

	suite.reader.AddContext(signalerCtx)

	fn()
}
