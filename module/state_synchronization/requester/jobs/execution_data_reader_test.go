package jobs

import (
	"context"
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/state_synchronization"
	syncmock "github.com/onflow/flow-go/module/state_synchronization/mock"
	synctest "github.com/onflow/flow-go/module/state_synchronization/requester/unittest"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type ExecutionDataReaderSuite struct {
	suite.Suite

	jobs         []module.Job
	reader       *ExecutionDataReader
	eds          *syncmock.ExecutionDataService
	headers      *storagemock.Headers
	results      *storagemock.ExecutionResults
	fetchTimeout time.Duration

	executionDataID flow.Identifier
	executionData   *state_synchronization.ExecutionData
	block           *flow.Block
	blocksByHeight  map[uint64]*flow.Block

	highestAvailableHeight func() uint64
}

func TestExecutionDataReaderSuite(t *testing.T) {
	t.Parallel()
	rand.Seed(time.Now().UnixMilli())
	suite.Run(t, new(ExecutionDataReaderSuite))
}

func (suite *ExecutionDataReaderSuite) SetupTest() {
	suite.fetchTimeout = time.Second
	suite.executionDataID = unittest.IdentifierFixture()

	parent := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(1))
	suite.block = unittest.BlockWithParentFixture(&parent)

	suite.executionData = synctest.ExecutionDataFixture(suite.block.ID())

	suite.highestAvailableHeight = func() uint64 { return suite.block.Header.Height + 1 }

	suite.reset()
}

func (suite *ExecutionDataReaderSuite) reset() {
	result := unittest.ExecutionResultFixture(
		unittest.WithBlock(suite.block),
		unittest.WithExecutionDataID(suite.executionDataID),
	)

	suite.blocksByHeight = map[uint64]*flow.Block{
		suite.block.Header.Height: suite.block,
	}
	suite.headers = synctest.MockBlockHeaderStorage(synctest.WithByHeight(suite.blocksByHeight))
	suite.results = synctest.MockResultsStorage(synctest.WithByBlockID(map[flow.Identifier]*flow.ExecutionResult{
		suite.block.ID(): result,
	}))

	suite.eds = new(syncmock.ExecutionDataService)
	suite.reader = NewExecutionDataReader(
		suite.eds,
		suite.headers,
		suite.results,
		suite.fetchTimeout,
		func() uint64 {
			return suite.highestAvailableHeight()
		},
	)
}

func (suite *ExecutionDataReaderSuite) TestAtIndex() {
	setExecutionDataGet := func(executionData *state_synchronization.ExecutionData, err error) {
		suite.eds.On("Get", mock.Anything, suite.executionDataID).Return(
			func(ctx context.Context, id flow.Identifier) *state_synchronization.ExecutionData {
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
			ed := synctest.ExecutionDataFixture(unittest.IdentifierFixture())
			setExecutionDataGet(ed, nil)

			job, err := suite.reader.AtIndex(suite.block.Header.Height)
			require.NoError(suite.T(), err)

			entry, err := JobToBlockEntry(job)
			assert.NoError(suite.T(), err)

			assert.Equal(suite.T(), entry.ExecutionData, ed)
		})
	})

	suite.Run("returns error from ExecutionDataService Get", func() {
		suite.reset()
		suite.runTest(func() {
			// return an error while getting the execution data
			expecteErr := errors.New("expected error: get failed")
			setExecutionDataGet(nil, expecteErr)

			job, err := suite.reader.AtIndex(suite.block.Header.Height)
			assert.Nil(suite.T(), job, "job should be nil")
			assert.ErrorIs(suite.T(), err, expecteErr)
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

	signalCtx, errChan := irrecoverable.WithSignaler(ctx)
	go irrecoverableNotExpected(suite.T(), ctx, errChan)

	suite.reader.AddContext(signalCtx)

	fn()
}

func irrecoverableNotExpected(t *testing.T, ctx context.Context, errChan <-chan error) {
	select {
	case <-ctx.Done():
		return
	case err := <-errChan:
		require.NoError(t, err, "unexpected irrecoverable error")
	}
}
