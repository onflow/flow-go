package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	txerrmsgsmock "github.com/onflow/flow-go/engine/access/ingestion/tx_error_messages/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	reqestermock "github.com/onflow/flow-go/module/state_synchronization/requester/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// CoreImplOption is a function type for configuring test CoreImpl instances.
type CoreImplOption func(*coreImplConfig)

// coreImplConfig holds configuration options for creating test CoreImpl instances.
type coreImplConfig struct {
	db storage.DB
}

// WithDB sets a custom database for the CoreImpl instance.
func WithDB(db storage.DB) CoreImplOption {
	return func(config *coreImplConfig) {
		config.db = db
	}
}

// createTestCoreImpl creates a CoreImpl instance with mocked dependencies for testing.
// It accepts optional configuration functions to customize the setup.
// It returns the core implementation along with mocked requesters for setting up test expectations.
func createTestCoreImpl(t *testing.T, opts ...CoreImplOption) (*CoreImpl, *reqestermock.ExecutionDataRequester, *txerrmsgsmock.TransactionResultErrorMessageRequester) {
	logger := zerolog.Nop()
	executionResult := unittest.ExecutionResultFixture()
	header := unittest.BlockHeaderFixture()

	execDataRequester := reqestermock.NewExecutionDataRequester(t)
	txResultErrMsgsRequester := txerrmsgsmock.NewTransactionResultErrorMessageRequester(t)

	// Apply configuration options
	config := &coreImplConfig{
		db: storagemock.NewDB(t), // default DB
	}
	for _, opt := range opts {
		opt(config)
	}

	// Create storage mocks with proper expectations for persist operations
	persistentRegisters := storagemock.NewRegisterIndex(t)
	persistentEvents := storagemock.NewEvents(t)
	persistentCollections := storagemock.NewCollections(t)
	persistentTransactions := storagemock.NewTransactions(t)
	persistentResults := storagemock.NewLightTransactionResults(t)
	persistentTxResultErrMsg := storagemock.NewTransactionResultErrorMessages(t)

	// Set up default expectations for persist operations
	// These will be called by the real Persister during Persist()
	persistentRegisters.On("Store", mock.Anything, mock.Anything).Return(nil).Maybe()
	persistentEvents.On("BatchStore", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	persistentCollections.On("BatchStoreLightAndIndexByTransaction", mock.Anything, mock.Anything).Return(nil).Maybe()
	persistentTransactions.On("BatchStore", mock.Anything, mock.Anything).Return(nil).Maybe()
	persistentResults.On("BatchStore", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	persistentTxResultErrMsg.On("BatchStore", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	core, err := NewCoreImpl(
		logger,
		executionResult,
		header,
		execDataRequester,
		txResultErrMsgsRequester,
		persistentRegisters,
		persistentEvents,
		persistentCollections,
		persistentTransactions,
		persistentResults,
		persistentTxResultErrMsg,
		config.db, // Use the configured DB
	)
	require.NoError(t, err)

	return core, execDataRequester, txResultErrMsgsRequester
}

func assertCoreInitialized(t *testing.T, core *CoreImpl) {
	require.NotNil(t, core)
	assert.NotNil(t, core.execDataRequester)
	assert.NotNil(t, core.txResultErrMsgsRequester)
	assert.NotNil(t, core.executionResult)
	assert.NotNil(t, core.header)
	assert.NotNil(t, core.inmemRegisters)
	assert.NotNil(t, core.inmemEvents)
	assert.NotNil(t, core.inmemCollections)
	assert.NotNil(t, core.inmemTransactions)
	assert.NotNil(t, core.inmemResults)
	assert.NotNil(t, core.inmemTxResultErrMsgs)
	assert.NotNil(t, core.indexer)
	assert.NotNil(t, core.persister)
}

// TestNewCoreImpl tests the successful creation of a CoreImpl instance.
func TestNewCoreImpl(t *testing.T) {
	core, _, _ := createTestCoreImpl(t)

	assertCoreInitialized(t, core)
}

// TestCoreImpl_Download tests the Download method which retrieves execution data and transaction error messages.
func TestCoreImpl_Download(t *testing.T) {
	t.Run("successful download", func(t *testing.T) {
		core, execDataRequester, txResultErrMsgsRequester := createTestCoreImpl(t)
		ctx := context.Background()

		expectedExecutionData := unittest.BlockExecutionDataFixture()
		execDataRequester.On("RequestExecutionData", ctx).Return(expectedExecutionData, nil)

		expectedTxResultErrMsgs := []flow.TransactionResultErrorMessage{
			{
				TransactionID: unittest.IdentifierFixture(),
				Index:         0,
				ErrorMessage:  "test error",
				ExecutorID:    unittest.IdentifierFixture(),
			},
		}
		txResultErrMsgsRequester.On("Request", ctx).Return(expectedTxResultErrMsgs, nil)

		err := core.Download(ctx)

		require.NoError(t, err)
		assert.Equal(t, expectedExecutionData, core.executionData.BlockExecutionData)
		assert.Equal(t, expectedTxResultErrMsgs, core.txResultErrMsgsData)
	})

	t.Run("execution data request error", func(t *testing.T) {
		core, execDataRequester, txResultErrMsgsRequester := createTestCoreImpl(t)
		ctx := context.Background()

		execDataRequester.On("RequestExecutionData", ctx).Return((*execution_data.BlockExecutionData)(nil), assert.AnError)

		err := core.Download(ctx)

		assert.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.Nil(t, core.executionData)

		txResultErrMsgsRequester.AssertNotCalled(t, "Request")
	})

	t.Run("transaction result error messages request error", func(t *testing.T) {
		core, execDataRequester, txResultErrMsgsRequester := createTestCoreImpl(t)
		ctx := context.Background()

		expectedExecutionData := unittest.BlockExecutionDataFixture()
		execDataRequester.On("RequestExecutionData", ctx).Return(expectedExecutionData, nil)

		txResultErrMsgsRequester.On("Request", ctx).Return(([]flow.TransactionResultErrorMessage)(nil), assert.AnError)

		err := core.Download(ctx)

		assert.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.NotNil(t, core.executionData)
	})
}

// TestCoreImpl_Index tests the Index method which processes downloaded data using the real InMemoryIndexer.
// The Index method must be called after Download has successfully retrieved execution data.
// Calling Index without prior successful Download will result in a panic due to nil execution data.
func TestCoreImpl_Index(t *testing.T) {
	t.Run("successful indexing", func(t *testing.T) {
		core, _, _ := createTestCoreImpl(t)
		ctx := context.Background()

		// Create execution data with the SAME block ID as the execution result
		executionData := unittest.BlockExecutionDataFixture(
			unittest.WithBlockExecutionDataBlockID(core.executionResult.BlockID),
		)
		core.executionData = execution_data.NewBlockExecutionDataEntity(
			core.executionResult.ExecutionDataID,
			executionData,
		)

		core.txResultErrMsgsData = []flow.TransactionResultErrorMessage{
			{
				TransactionID: unittest.IdentifierFixture(),
				Index:         0,
				ErrorMessage:  "test error",
				ExecutorID:    unittest.IdentifierFixture(),
			},
		}

		err := core.Index(ctx)
		require.NoError(t, err)
	})

	t.Run("block ID mismatch", func(t *testing.T) {
		core, _, _ := createTestCoreImpl(t)
		ctx := context.Background()

		// Create execution data with a DIFFERENT block ID than expected
		executionData := unittest.BlockExecutionDataFixture()
		core.executionData = execution_data.NewBlockExecutionDataEntity(
			unittest.IdentifierFixture(),
			executionData,
		)

		err := core.Index(ctx)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid block execution data")
		assert.Contains(t, err.Error(), "expected block_id")
	})
}

// TestCoreImpl_Persist tests the Persist method which saves indexed data using the real Persister.
func TestCoreImpl_Persist(t *testing.T) {
	t.Run("successful persistence", func(t *testing.T) {
		// Create mocks with proper expectations
		mockDB := storagemock.NewDB(t)
		mockBatch := storagemock.NewBatch(t)

		// Set up successful batch operations
		mockBatch.On("Commit").Return(nil)
		mockBatch.On("Close").Return(nil).Maybe()
		mockDB.On("NewBatch").Return(mockBatch)

		core, _, _ := createTestCoreImpl(t, WithDB(mockDB))
		ctx := context.Background()

		err := core.Persist(ctx)

		require.NoError(t, err)
	})

	t.Run("persistence with batch commit failure", func(t *testing.T) {
		// Create a failing DB
		mockDB := storagemock.NewDB(t)
		mockBatch := storagemock.NewBatch(t)

		mockBatch.On("Commit").Return(errors.New("batch commit failed"))
		mockBatch.On("Close").Return(nil).Maybe()
		mockDB.On("NewBatch").Return(mockBatch)

		// Create CoreImpl with the failing DB
		core, _, _ := createTestCoreImpl(t, WithDB(mockDB))
		ctx := context.Background()

		err := core.Persist(ctx)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to persist data")
		assert.Contains(t, err.Error(), "batch commit failed")
	})
}

// TestCoreImpl_Abandon tests the Abandon method which clears all references for garbage collection.
func TestCoreImpl_Abandon(t *testing.T) {
	t.Run("successful abandon", func(t *testing.T) {
		core, _, _ := createTestCoreImpl(t)
		ctx := context.Background()

		core.executionData = unittest.BlockExecutionDatEntityFixture()
		core.txResultErrMsgsData = []flow.TransactionResultErrorMessage{
			{
				TransactionID: unittest.IdentifierFixture(),
				Index:         0,
				ErrorMessage:  "test error",
				ExecutorID:    unittest.IdentifierFixture(),
			},
		}

		err := core.Abandon(ctx)

		require.NoError(t, err)

		assert.Nil(t, core.inmemRegisters)
		assert.Nil(t, core.inmemEvents)
		assert.Nil(t, core.inmemCollections)
		assert.Nil(t, core.inmemTransactions)
		assert.Nil(t, core.inmemResults)
		assert.Nil(t, core.inmemTxResultErrMsgs)
		assert.Nil(t, core.txResultErrMsgsRequester)
		assert.Nil(t, core.execDataRequester)
		assert.Nil(t, core.indexer)
		assert.Nil(t, core.persister)
		assert.Nil(t, core.executionData)
		assert.Nil(t, core.txResultErrMsgsData)
	})
}

// TestCoreImpl_IntegrationWorkflow tests the complete workflow of download -> index -> persist operations.
func TestCoreImpl_IntegrationWorkflow(t *testing.T) {
	t.Run("complete workflow - download, index, persist", func(t *testing.T) {
		// Set up mocks with proper expectations
		mockDB := storagemock.NewDB(t)
		mockBatch := storagemock.NewBatch(t)

		// Set up successful batch operations
		mockBatch.On("Commit").Return(nil)
		mockBatch.On("Close").Return(nil).Maybe()
		mockDB.On("NewBatch").Return(mockBatch)

		core, execDataRequester, txResultErrMsgsRequester := createTestCoreImpl(t, WithDB(mockDB))
		ctx := context.Background()

		// Create execution data with the SAME block ID as the execution result
		executionData := unittest.BlockExecutionDataFixture(
			unittest.WithBlockExecutionDataBlockID(core.executionResult.BlockID),
		)
		txResultErrMsgs := []flow.TransactionResultErrorMessage{
			{
				TransactionID: unittest.IdentifierFixture(),
				Index:         0,
				ErrorMessage:  "test error",
				ExecutorID:    unittest.IdentifierFixture(),
			},
		}

		execDataRequester.On("RequestExecutionData", ctx).Return(executionData, nil)
		txResultErrMsgsRequester.On("Request", ctx).Return(txResultErrMsgs, nil)

		err := core.Download(ctx)
		require.NoError(t, err)

		err = core.Index(ctx)
		require.NoError(t, err)

		err = core.Persist(ctx)
		require.NoError(t, err)

		assert.NotNil(t, core.executionData)
		assert.Equal(t, executionData, core.executionData.BlockExecutionData)
		assert.Equal(t, txResultErrMsgs, core.txResultErrMsgsData)
	})
}

// TestCoreImpl_ErrorScenarios tests various error conditions and edge cases.
// These tests validate that the CoreImpl handles invalid states and usage patterns correctly,
// typically by panicking for programming errors or returning appropriate errors for operational issues.
func TestCoreImpl_ErrorScenarios(t *testing.T) {
	t.Run("context cancellation during download", func(t *testing.T) {
		core, execDataRequester, _ := createTestCoreImpl(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		execDataRequester.On("RequestExecutionData", ctx).Return((*execution_data.BlockExecutionData)(nil), context.Canceled)

		err := core.Download(ctx)

		assert.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
	})
}
