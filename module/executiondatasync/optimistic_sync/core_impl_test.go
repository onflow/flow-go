package optimistic_sync

import (
	"context"
	"testing"
	"time"

	"github.com/jordanschalm/lockctx"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	txerrmsgsmock "github.com/onflow/flow-go/engine/access/ingestion/tx_error_messages/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	reqestermock "github.com/onflow/flow-go/module/state_synchronization/requester/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// CoreImplSuite is a test suite for testing the CoreImpl.
type CoreImplSuite struct {
	suite.Suite
	logger                        zerolog.Logger
	execDataRequester             *reqestermock.ExecutionDataRequester
	txResultErrMsgsRequester      *txerrmsgsmock.Requester
	txResultErrMsgsRequestTimeout time.Duration
	db                            *storagemock.DB
	lockManager                   lockctx.Manager
	persistentRegisters           *storagemock.RegisterIndex
	persistentEvents              *storagemock.Events
	persistentCollections         *storagemock.Collections
	persistentTransactions        *storagemock.Transactions
	persistentResults             *storagemock.LightTransactionResults
	persistentTxResultErrMsg      *storagemock.TransactionResultErrorMessages
	latestPersistedSealedResult   *storagemock.LatestPersistedSealedResult
}

func TestCoreImplSuiteSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(CoreImplSuite))
}

func (c *CoreImplSuite) SetupTest() {
	c.lockManager = storage.NewTestingLockManager()
	t := c.T()
	c.logger = zerolog.Nop()

	c.execDataRequester = reqestermock.NewExecutionDataRequester(t)
	c.txResultErrMsgsRequester = txerrmsgsmock.NewRequester(t)
	c.txResultErrMsgsRequestTimeout = DefaultTxResultErrMsgsRequestTimeout

	c.db = storagemock.NewDB(t)
	c.db.On("WithReaderBatchWriter", mock.Anything).Return(
		func(fn func(storage.ReaderBatchWriter) error) error {
			return fn(storagemock.NewBatch(t))
		},
	).Maybe()

	// Create storage mocks with proper expectations for persist operations
	c.persistentRegisters = storagemock.NewRegisterIndex(t)
	c.persistentEvents = storagemock.NewEvents(t)
	c.persistentCollections = storagemock.NewCollections(t)
	c.persistentTransactions = storagemock.NewTransactions(t)
	c.persistentResults = storagemock.NewLightTransactionResults(t)
	c.persistentTxResultErrMsg = storagemock.NewTransactionResultErrorMessages(t)
	c.latestPersistedSealedResult = storagemock.NewLatestPersistedSealedResult(t)

	// Set up default expectations for persist operations
	// These will be called by the real Persister during Persist()
	c.persistentRegisters.On("Store", mock.Anything, mock.Anything).Return(nil).Maybe()
	c.persistentEvents.On("BatchStore", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	c.persistentCollections.On("BatchStoreLightAndIndexByTransaction", mock.Anything, mock.Anything).Return(nil).Maybe()
	c.persistentTransactions.On("BatchStore", mock.Anything, mock.Anything).Return(nil).Maybe()
	c.persistentResults.On("BatchStore", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	c.persistentTxResultErrMsg.On("BatchStore", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	c.latestPersistedSealedResult.On("BatchSet", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
}

// createTestCoreImpl creates a CoreImpl instance with mocked dependencies for testing.
//
// Returns a configured CoreImpl ready for testing.
func (c *CoreImplSuite) createTestCoreImpl() *CoreImpl {
	block := unittest.BlockFixture()
	executionResult := unittest.ExecutionResultFixture(unittest.WithBlock(block))

	return NewCoreImpl(
		c.logger,
		executionResult,
		block.ToHeader(),
		c.execDataRequester,
		c.txResultErrMsgsRequester,
		c.txResultErrMsgsRequestTimeout,
		c.persistentRegisters,
		c.persistentEvents,
		c.persistentCollections,
		c.persistentResults,
		c.persistentTxResultErrMsg,
		c.latestPersistedSealedResult,
		c.db,
		c.lockManager,
	)
}

// TestCoreImpl_Download tests the Download method which retrieves execution data and transaction error messages.
func (c *CoreImplSuite) TestCoreImpl_Download() {
	c.Run("successful download", func() {
		core := c.createTestCoreImpl()
		ctx := context.Background()

		expectedExecutionData := unittest.BlockExecutionDataFixture(unittest.WithBlockExecutionDataBlockID(core.header.ID()))
		c.execDataRequester.On("RequestExecutionData", mock.Anything).Return(expectedExecutionData, nil).Once()

		expectedTxResultErrMsgs := unittest.TransactionResultErrorMessagesFixture(1)
		c.txResultErrMsgsRequester.On("Request", mock.Anything).Return(expectedTxResultErrMsgs, nil).Once()

		err := core.Download(ctx)
		c.Require().NoError(err)

		c.Assert().Equal(expectedExecutionData, core.workingData.executionData.BlockExecutionData)
		c.Assert().Equal(expectedTxResultErrMsgs, core.workingData.txResultErrMsgsData)
	})

	c.Run("execution data request error", func() {
		c.execDataRequester.On("RequestExecutionData", mock.Anything).Return((*execution_data.BlockExecutionData)(nil), assert.AnError).Once()
		c.txResultErrMsgsRequester.On("Request", mock.Anything).Return(([]flow.TransactionResultErrorMessage)(nil), nil).Once()

		ctx := context.Background()
		core := c.createTestCoreImpl()
		err := core.Download(ctx)
		c.Require().Error(err)

		c.Assert().ErrorIs(err, assert.AnError)
		c.Assert().Contains(err.Error(), "failed to request execution data")
		c.Assert().Nil(core.workingData.executionData)
		c.Assert().Nil(core.workingData.txResultErrMsgsData)
	})

	c.Run("transaction result error messages request error", func() {
		expectedExecutionData := unittest.BlockExecutionDataFixture()
		c.execDataRequester.On("RequestExecutionData", mock.Anything).Return(expectedExecutionData, nil).Once()
		c.txResultErrMsgsRequester.On("Request", mock.Anything).Return(([]flow.TransactionResultErrorMessage)(nil), assert.AnError).Once()

		ctx := context.Background()
		core := c.createTestCoreImpl()

		err := core.Download(ctx)
		c.Require().Error(err)

		c.Assert().ErrorIs(err, assert.AnError)
		c.Assert().Contains(err.Error(), "failed to request transaction result error messages data")
		c.Assert().Nil(core.workingData.executionData)
		c.Assert().Nil(core.workingData.txResultErrMsgsData)
	})

	c.Run("context cancellation", func() {
		core := c.createTestCoreImpl()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		expectedExecutionData := unittest.BlockExecutionDataFixture()
		c.execDataRequester.On("RequestExecutionData", mock.Anything).Return(expectedExecutionData, ctx.Err()).Once()

		expectedTxResultErrMsgs := unittest.TransactionResultErrorMessagesFixture(1)
		c.txResultErrMsgsRequester.On("Request", mock.Anything).Return(expectedTxResultErrMsgs, ctx.Err()).Once()

		err := core.Download(ctx)
		c.Require().Error(err)

		c.Assert().ErrorIs(err, context.Canceled)
		c.Assert().Nil(core.workingData.executionData)
		c.Assert().Nil(core.workingData.txResultErrMsgsData)
	})

	c.Run("txResultErrMsgsRequestTimeout expiration", func() {
		c.txResultErrMsgsRequestTimeout = 100 * time.Millisecond

		expectedExecutionData := unittest.BlockExecutionDataFixture()
		c.execDataRequester.On("RequestExecutionData", mock.Anything).Return(expectedExecutionData, nil).Once()

		// Transaction result error messages request times out
		c.txResultErrMsgsRequester.On("Request", mock.MatchedBy(func(ctx context.Context) bool {
			// Verify we received a context with timeout
			deadline, hasDeadline := ctx.Deadline()
			if !hasDeadline {
				return false
			}
			// Verify the timeout is approximately what we expect
			timeUntilDeadline := time.Until(deadline)
			return timeUntilDeadline > 0 && timeUntilDeadline <= c.txResultErrMsgsRequestTimeout
		})).Run(func(args mock.Arguments) {
			// Simulate a slow request by sleeping longer than the timeout
			time.Sleep(2 * c.txResultErrMsgsRequestTimeout)
		}).Return(([]flow.TransactionResultErrorMessage)(nil), context.DeadlineExceeded).Once()

		core := c.createTestCoreImpl()
		ctx := context.Background()

		var err error
		unittest.AssertReturnsBefore(c.T(), func() {
			err = core.Download(ctx)
		}, time.Second)

		c.Require().NoError(err)
		c.Assert().Equal(expectedExecutionData, core.workingData.executionData.BlockExecutionData)
		c.Assert().Nil(core.workingData.txResultErrMsgsData)
	})
}

// TestCoreImpl_Index tests the Index method which processes downloaded data.
func (c *CoreImplSuite) TestCoreImpl_Index() {
	c.Run("successful indexing", func() {
		core := c.createTestCoreImpl()

		// Create execution data with the SAME block ID as the execution result
		expectedExecutionData := unittest.BlockExecutionDataFixture(
			unittest.WithBlockExecutionDataBlockID(core.executionResult.BlockID),
		)
		c.execDataRequester.On("RequestExecutionData", mock.Anything).Return(expectedExecutionData, nil).Once()

		expectedTxResultErrMsgs := unittest.TransactionResultErrorMessagesFixture(1)
		c.txResultErrMsgsRequester.On("Request", mock.Anything).Return(expectedTxResultErrMsgs, nil).Once()

		ctx := context.Background()
		err := core.Download(ctx)
		c.Require().NoError(err)

		err = core.Index()
		c.Require().NoError(err)
	})

	c.Run("block ID mismatch", func() {
		core := c.createTestCoreImpl()

		// Create execution data with a DIFFERENT block ID than expected
		executionData := unittest.BlockExecutionDataFixture()
		core.workingData.executionData = execution_data.NewBlockExecutionDataEntity(
			unittest.IdentifierFixture(),
			executionData,
		)

		err := core.Index()
		c.Require().Error(err)

		c.Assert().Contains(err.Error(), "invalid block execution data")
		c.Assert().Contains(err.Error(), "expected block_id")
	})

	c.Run("execution data is empty", func() {
		core := c.createTestCoreImpl()

		// Do not download data, just index it
		err := core.Index()
		c.Require().Error(err)

		c.Assert().Contains(err.Error(), "could not index an empty execution data")
	})
}

// TestCoreImpl_Persist tests the Persist method which persists indexed data to storages and database.
func (c *CoreImplSuite) TestCoreImpl_Persist() {
	t := c.T()

	c.Run("successful persistence of empty data", func() {
		// Create mocks with proper expectations
		c.db = storagemock.NewDB(t)
		c.db.On("WithReaderBatchWriter", mock.Anything).Return(nil)

		core := c.createTestCoreImpl()
		err := core.Persist()

		c.Require().NoError(err)
	})

	c.Run("persistence with batch commit failure", func() {
		// Create a failing DB
		c.db = storagemock.NewDB(t)
		c.db.On("WithReaderBatchWriter", mock.Anything).Return(assert.AnError)

		// Create CoreImpl with the failing DB
		core := c.createTestCoreImpl()

		err := core.Persist()
		c.Require().Error(err)

		c.Assert().ErrorIs(err, assert.AnError)
		c.Assert().Contains(err.Error(), "failed to persist block data")
	})
}

// TestCoreImpl_Abandon tests the Abandon method which clears all references for garbage collection.
func (c *CoreImplSuite) TestCoreImpl_Abandon() {
	core := c.createTestCoreImpl()

	core.workingData.executionData = unittest.BlockExecutionDatEntityFixture()
	core.workingData.txResultErrMsgsData = unittest.TransactionResultErrorMessagesFixture(1)

	err := core.Abandon()
	c.Require().NoError(err)

	c.Assert().Nil(core.workingData)
}

// TestCoreImpl_IntegrationWorkflow tests the complete workflow of download -> index -> persist operations.
func (c *CoreImplSuite) TestCoreImpl_IntegrationWorkflow() {
	t := c.T()

	// Set up mocks with proper expectations
	c.db = storagemock.NewDB(t)
	c.db.On("WithReaderBatchWriter", mock.Anything).Return(
		func(fn func(storage.ReaderBatchWriter) error) error {
			return fn(storagemock.NewBatch(t))
		},
	).Maybe()

	core := c.createTestCoreImpl()
	ctx := context.Background()

	// Create execution data with the SAME block ID as the execution result
	executionData := unittest.BlockExecutionDataFixture(
		unittest.WithBlockExecutionDataBlockID(core.executionResult.BlockID),
	)
	txResultErrMsgs := unittest.TransactionResultErrorMessagesFixture(1)

	c.execDataRequester.On("RequestExecutionData", mock.Anything).Return(executionData, nil).Once()
	c.txResultErrMsgsRequester.On("Request", mock.Anything).Return(txResultErrMsgs, nil).Once()

	err := core.Download(ctx)
	c.Require().NoError(err)

	err = core.Index()
	c.Require().NoError(err)

	err = core.Persist()
	c.Require().NoError(err)

	c.Assert().NotNil(core.workingData.executionData)
	c.Assert().Equal(executionData, core.workingData.executionData.BlockExecutionData)
	c.Assert().Equal(txResultErrMsgs, core.workingData.txResultErrMsgsData)
}
