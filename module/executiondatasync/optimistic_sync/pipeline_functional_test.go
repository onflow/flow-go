package optimistic_sync

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jordanschalm/lockctx"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	txerrmsgsmock "github.com/onflow/flow-go/engine/access/ingestion/tx_error_messages/mock"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	reqestermock "github.com/onflow/flow-go/module/state_synchronization/requester/mock"
	"github.com/onflow/flow-go/storage"
	bstorage "github.com/onflow/flow-go/storage/badger"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	pebbleStorage "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

type PipelineFunctionalSuite struct {
	suite.Suite
	logger                        zerolog.Logger
	execDataRequester             *reqestermock.ExecutionDataRequester
	txResultErrMsgsRequester      *txerrmsgsmock.Requester
	txResultErrMsgsRequestTimeout time.Duration
	tmpDir                        string
	registerTmpDir                string
	registerDB                    storage.DB
	db                            storage.DB
	lockManager                   lockctx.Manager
	persistentRegisters           *pebbleStorage.Registers
	persistentEvents              storage.Events
	persistentCollections         *store.Collections
	persistentTransactions        *store.Transactions
	persistentResults             *store.LightTransactionResults
	persistentTxResultErrMsg      *store.TransactionResultErrorMessages
	consumerProgress              storage.ConsumerProgress
	headers                       *store.Headers
	results                       *store.ExecutionResults
	persistentLatestSealedResult  *store.LatestPersistedSealedResult
	core                          *CoreImpl
	block                         *flow.Block
	executionResult               *flow.ExecutionResult
	metrics                       module.CacheMetrics
	config                        PipelineConfig
	expectedExecutionData         *execution_data.BlockExecutionData
	expectedTxResultErrMsgs       []flow.TransactionResultErrorMessage
}

func TestPipelineFunctionalSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(PipelineFunctionalSuite))
}

// SetupTest initializes the test environment for each test case.
// It creates temporary directories, initializes database connections,
// sets up storage backends, creates test fixtures, and initializes
// the core implementation with all required dependencies.
func (p *PipelineFunctionalSuite) SetupTest() {
	t := p.T()
	p.lockManager = storage.NewTestingLockManager()

	p.tmpDir = unittest.TempDir(t)
	p.logger = zerolog.Nop()
	p.metrics = metrics.NewNoopCollector()
	pdb := unittest.PebbleDB(t, p.tmpDir)
	p.db = pebbleimpl.ToDB(pdb)

	rootBlock := unittest.BlockHeaderFixture()
	sealedBlock := unittest.BlockWithParentFixture(rootBlock)
	sealedExecutionResult := unittest.ExecutionResultFixture(unittest.WithBlock(sealedBlock))

	// Create real storages
	var err error
	// Use a separate directory for the register database to avoid lock conflicts
	p.registerTmpDir = unittest.TempDir(t)
	registerDB := pebbleStorage.NewBootstrappedRegistersWithPathForTest(t, p.registerTmpDir, rootBlock.Height, sealedBlock.Height)
	p.registerDB = pebbleimpl.ToDB(registerDB)
	p.persistentRegisters, err = pebbleStorage.NewRegisters(registerDB, pebbleStorage.PruningDisabled)
	p.Require().NoError(err)

	p.persistentEvents = store.NewEvents(p.metrics, p.db)
	p.persistentTransactions = store.NewTransactions(p.metrics, p.db)
	p.persistentCollections = store.NewCollections(p.db, p.persistentTransactions)
	p.persistentResults = store.NewLightTransactionResults(p.metrics, p.db, bstorage.DefaultCacheSize)
	p.persistentTxResultErrMsg = store.NewTransactionResultErrorMessages(p.metrics, p.db, bstorage.DefaultCacheSize)
	p.results = store.NewExecutionResults(p.metrics, p.db)

	p.consumerProgress, err = store.NewConsumerProgress(p.db, "test_consumer").Initialize(sealedBlock.Height)
	p.Require().NoError(err)

	// store and index the root header
	p.headers = store.NewHeaders(p.metrics, p.db)

	insertLctx := p.lockManager.NewContext()
	err = insertLctx.AcquireLock(storage.LockInsertBlock)
	p.Require().NoError(err)

	err = p.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return operation.InsertHeader(insertLctx, rw, rootBlock.ID(), rootBlock)
	})
	p.Require().NoError(err)
	insertLctx.Release()

	lctx := p.lockManager.NewContext()
	require.NoError(t, lctx.AcquireLock(storage.LockFinalizeBlock))
	err = p.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return operation.IndexFinalizedBlockByHeight(lctx, rw, rootBlock.Height, rootBlock.ID())
	})
	p.Require().NoError(err)
	lctx.Release()

	// store and index the latest sealed block header
	insertLctx2 := p.lockManager.NewContext()
	require.NoError(t, insertLctx2.AcquireLock(storage.LockInsertBlock))
	err = p.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return operation.InsertHeader(insertLctx2, rw, sealedBlock.ID(), sealedBlock.ToHeader())
	})
	p.Require().NoError(err)
	insertLctx2.Release()

	lctx = p.lockManager.NewContext()
	require.NoError(t, lctx.AcquireLock(storage.LockFinalizeBlock))
	err = p.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return operation.IndexFinalizedBlockByHeight(lctx, rw, sealedBlock.Height, sealedBlock.ID())
	})
	p.Require().NoError(err)
	lctx.Release()

	// Store and index sealed block execution result
	err = p.results.Store(sealedExecutionResult)
	p.Require().NoError(err)

	err = p.results.Index(sealedBlock.ID(), sealedExecutionResult.ID())
	p.Require().NoError(err)

	p.persistentLatestSealedResult, err = store.NewLatestPersistedSealedResult(p.consumerProgress, p.headers, p.results)
	p.Require().NoError(err)

	p.block = unittest.BlockWithParentFixture(sealedBlock.ToHeader())
	p.executionResult = unittest.ExecutionResultFixture(unittest.WithBlock(p.block))

	p.execDataRequester = reqestermock.NewExecutionDataRequester(t)
	p.txResultErrMsgsRequester = txerrmsgsmock.NewRequester(t)
	p.txResultErrMsgsRequestTimeout = DefaultTxResultErrMsgsRequestTimeout

	p.config = PipelineConfig{
		parentState: StateWaitingPersist,
	}
	p.expectedExecutionData, p.expectedTxResultErrMsgs = p.createExecutionData()
}

// TearDownTest cleans up resources after each test case.
// It closes database connections and removes temporary directories
// to ensure a clean state for subsequent tests.
func (p *PipelineFunctionalSuite) TearDownTest() {
	p.Require().NoError(p.db.Close())
	p.Require().NoError(p.registerDB.Close())
	p.Require().NoError(os.RemoveAll(p.tmpDir))
	p.Require().NoError(os.RemoveAll(p.registerTmpDir))
}

// TestPipelineCompletesSuccessfully verifies the successful completion of the pipeline.
// It tests that:
// 1. Pipeline processes execution data through all states correctly
// 2. All data types (events, collections, transactions, registers, error messages) are correctly persisted to storage
// 3. No errors occur during the entire process
func (p *PipelineFunctionalSuite) TestPipelineCompletesSuccessfully() {
	p.execDataRequester.On("RequestExecutionData", mock.Anything).Return(p.expectedExecutionData, nil).Once()
	p.txResultErrMsgsRequester.On("Request", mock.Anything).Return(p.expectedTxResultErrMsgs, nil).Once()

	p.WithRunningPipeline(func(pipeline Pipeline, updateChan chan State, errChan chan error, cancel context.CancelFunc) {
		pipeline.OnParentStateUpdated(StateComplete)

		waitForStateUpdates(p.T(), updateChan, StateProcessing, StateWaitingPersist)

		pipeline.SetSealed()

		waitForStateUpdates(p.T(), updateChan, StateComplete)

		expectedChunkExecutionData := p.expectedExecutionData.ChunkExecutionDatas[0]
		p.verifyDataPersistence(expectedChunkExecutionData, p.expectedTxResultErrMsgs)
	}, p.config)
}

// TestPipelineDownloadError tests how the pipeline handles errors during the download phase.
// It ensures that both execution data and transaction result error message request errors
// are correctly detected and returned.
func (p *PipelineFunctionalSuite) TestPipelineDownloadError() {
	tests := []struct {
		name                    string
		expectedErr             error
		requesterInitialization func(err error)
	}{
		{
			name:        "execution data requester malformed data error",
			expectedErr: execution_data.NewMalformedDataError(fmt.Errorf("execution data test deserialization error")),
			requesterInitialization: func(err error) {
				p.execDataRequester.On("RequestExecutionData", mock.Anything).Return((*execution_data.BlockExecutionData)(nil), err).Once()
				p.txResultErrMsgsRequester.On("Request", mock.Anything).Return(p.expectedTxResultErrMsgs, nil).Once()
			},
		},
		{
			name:        "transaction result error messages requester not found error",
			expectedErr: fmt.Errorf("test transaction result error messages not found error"),
			requesterInitialization: func(err error) {
				p.execDataRequester.On("RequestExecutionData", mock.Anything).Return(p.expectedExecutionData, nil).Once()
				p.txResultErrMsgsRequester.On("Request", mock.Anything).Return(([]flow.TransactionResultErrorMessage)(nil), err).Once()
			},
		},
	}

	for _, test := range tests {
		p.T().Run(test.name, func(t *testing.T) {
			test.requesterInitialization(test.expectedErr)

			p.WithRunningPipeline(func(pipeline Pipeline, updateChan chan State, errChan chan error, cancel context.CancelFunc) {
				pipeline.OnParentStateUpdated(StateComplete)

				waitForError(p.T(), errChan, test.expectedErr)
				p.Assert().Equal(StateProcessing, pipeline.GetState())
			}, p.config)
		})
	}
}

// TestPipelineIndexingError tests error handling during the indexing phase.
// It verifies that when execution data contains invalid block IDs, the pipeline
// properly detects the inconsistency and returns an appropriate error.
func (p *PipelineFunctionalSuite) TestPipelineIndexingError() {
	invalidBlockID := unittest.IdentifierFixture()
	// Setup successful download
	expectedExecutionData := unittest.BlockExecutionDataFixture(
		unittest.WithBlockExecutionDataBlockID(invalidBlockID), // Wrong block ID to cause indexing error
	)
	p.execDataRequester.On("RequestExecutionData", mock.Anything).Return(expectedExecutionData, nil).Once()

	// note: txResultErrMsgsRequester.Request() currently never returns and error, so skipping the case
	p.txResultErrMsgsRequester.On("Request", mock.Anything).Return(p.expectedTxResultErrMsgs, nil).Once()

	expectedIndexingError := fmt.Errorf(
		"could not perform indexing: invalid block execution data. expected block_id=%s, actual block_id=%s",
		p.block.ID().String(),
		invalidBlockID.String(),
	)

	p.WithRunningPipeline(func(pipeline Pipeline, updateChan chan State, errChan chan error, cancel context.CancelFunc) {
		pipeline.OnParentStateUpdated(StateComplete)

		waitForErrorWithCustomCheckers(p.T(), errChan, func(err error) {
			p.Require().Error(err)

			p.Assert().Equal(expectedIndexingError.Error(), err.Error())
		})
		p.Assert().Equal(StateProcessing, pipeline.GetState())
	}, p.config)
}

// TestPipelinePersistingError tests the pipeline behavior when an error occurs during the persisting step.
func (p *PipelineFunctionalSuite) TestPipelinePersistingError() {
	expectedError := fmt.Errorf("test events batch store error")
	// Mock events storage to simulate an error on a persisting step. In normal flow and with real storages,
	// it is hard to make a meaningful error explicitly.
	mockEvents := storagemock.NewEvents(p.T())
	mockEvents.On("BatchStore", mock.Anything, mock.Anything, mock.Anything).Return(expectedError).Once()
	p.persistentEvents = mockEvents

	p.execDataRequester.On("RequestExecutionData", mock.Anything).Return(p.expectedExecutionData, nil).Once()
	p.txResultErrMsgsRequester.On("Request", mock.Anything).Return(p.expectedTxResultErrMsgs, nil).Once()

	p.WithRunningPipeline(func(pipeline Pipeline, updateChan chan State, errChan chan error, cancel context.CancelFunc) {
		pipeline.OnParentStateUpdated(StateComplete)

		waitForStateUpdates(p.T(), updateChan, StateProcessing, StateWaitingPersist)

		pipeline.SetSealed()

		waitForError(p.T(), errChan, expectedError)
		p.Assert().Equal(StateWaitingPersist, pipeline.GetState())
	}, p.config)
}

// TestMainCtxCancellationDuringRequestingExecutionData tests context cancellation during the
// request of execution data. It ensures that cancellation is handled properly when triggered
// while execution data is being downloaded.
func (p *PipelineFunctionalSuite) TestMainCtxCancellationDuringRequestingExecutionData() {
	p.WithRunningPipeline(func(pipeline Pipeline, updateChan chan State, errChan chan error, cancel context.CancelFunc) {
		p.execDataRequester.On("RequestExecutionData", mock.Anything).Return(
			func(ctx context.Context) (*execution_data.BlockExecutionData, error) {
				// Wait for cancellation
				cancel()

				<-ctx.Done()

				return nil, ctx.Err()
			}).Once()

		// This call marked as `Maybe()` because it may not be called depending on timing.
		p.txResultErrMsgsRequester.On("Request", mock.Anything).Return([]flow.TransactionResultErrorMessage{}, nil).Maybe()

		pipeline.OnParentStateUpdated(StateComplete)

		waitForStateUpdates(p.T(), updateChan, StateProcessing)
		waitForError(p.T(), errChan, context.Canceled)

		p.Assert().Equal(StateProcessing, pipeline.GetState())
	}, p.config)
}

// TestMainCtxCancellationDuringRequestingTxResultErrMsgs tests context cancellation during
// the request of transaction result error messages. It verifies that when the parent context
// is cancelled during this phase, the pipeline handles the cancellation gracefully
// and transitions to the correct state.
func (p *PipelineFunctionalSuite) TestMainCtxCancellationDuringRequestingTxResultErrMsgs() {
	p.WithRunningPipeline(func(pipeline Pipeline, updateChan chan State, errChan chan error, cancel context.CancelFunc) {
		// This call marked as `Maybe()` because it may not be called depending on timing.
		p.execDataRequester.On("RequestExecutionData", mock.Anything).Return((*execution_data.BlockExecutionData)(nil), nil).Maybe()

		p.txResultErrMsgsRequester.On("Request", mock.Anything).Return(
			func(ctx context.Context) ([]flow.TransactionResultErrorMessage, error) {
				// Wait for cancellation
				cancel()

				<-ctx.Done()

				return nil, ctx.Err()
			}).Maybe()

		pipeline.OnParentStateUpdated(StateComplete)

		waitForStateUpdates(p.T(), updateChan, StateProcessing)
		waitForError(p.T(), errChan, context.Canceled)

		p.Assert().Equal(StateProcessing, pipeline.GetState())
	}, p.config)
}

// TestMainCtxCancellationDuringWaitingPersist tests the pipeline's behavior when the main context is canceled during StateWaitingPersist.
func (p *PipelineFunctionalSuite) TestMainCtxCancellationDuringWaitingPersist() {
	p.execDataRequester.On("RequestExecutionData", mock.Anything).Return(p.expectedExecutionData, nil).Once()
	p.txResultErrMsgsRequester.On("Request", mock.Anything).Return(p.expectedTxResultErrMsgs, nil).Once()

	p.WithRunningPipeline(func(pipeline Pipeline, updateChan chan State, errChan chan error, cancel context.CancelFunc) {
		pipeline.OnParentStateUpdated(StateComplete)

		waitForStateUpdates(p.T(), updateChan, StateProcessing, StateWaitingPersist)

		cancel()

		pipeline.SetSealed()

		waitForError(p.T(), errChan, context.Canceled)

		p.Assert().Equal(StateWaitingPersist, pipeline.GetState())
	}, p.config)
}

// TestPipelineShutdownOnParentAbandon verifies that the pipeline transitions correctly to a shutdown state when the parent is abandoned.
func (p *PipelineFunctionalSuite) TestPipelineShutdownOnParentAbandon() {
	tests := []struct {
		name        string
		config      PipelineConfig
		customSetup func(pipeline Pipeline, updateChan chan State)
	}{
		{
			name: "from StatePending",
			config: PipelineConfig{
				beforePipelineRun: func(pipeline *PipelineImpl) {
					pipeline.OnParentStateUpdated(StateAbandoned)
				},
				parentState: StateAbandoned,
			},
		},
		{
			name: "from StateProcessing",
			customSetup: func(pipeline Pipeline, updateChan chan State) {
				waitForStateUpdates(p.T(), updateChan, StateProcessing)

				pipeline.OnParentStateUpdated(StateAbandoned)
			},
			config: p.config,
		},
		{
			name: "from StateWaitingPersist",
			customSetup: func(pipeline Pipeline, updateChan chan State) {
				waitForStateUpdates(p.T(), updateChan, StateProcessing, StateWaitingPersist)

				pipeline.OnParentStateUpdated(StateAbandoned)
			},
			config: p.config,
		},
	}

	for _, test := range tests {
		p.T().Run(test.name, func(t *testing.T) {
			p.WithRunningPipeline(func(pipeline Pipeline, updateChan chan State, errChan chan error, cancel context.CancelFunc) {
				p.execDataRequester.On("RequestExecutionData", mock.Anything).Return(p.expectedExecutionData, nil).Maybe()
				p.txResultErrMsgsRequester.On("Request", mock.Anything).Return(p.expectedTxResultErrMsgs, nil).Maybe()

				if test.customSetup != nil {
					test.customSetup(pipeline, updateChan)
				}

				waitForStateUpdates(p.T(), updateChan, StateAbandoned)
				waitForError(p.T(), errChan, nil)

				p.Assert().Equal(StateAbandoned, pipeline.GetState())
				p.Assert().Nil(p.core.workingData)
			}, test.config)
		})
	}
}

type PipelineConfig struct {
	beforePipelineRun func(pipeline *PipelineImpl)
	parentState       State
}

// WithRunningPipeline is a test helper that initializes and starts a pipeline instance.
// It manages the context and channels needed to run the pipeline and invokes the testFunc
// with access to the pipeline, update channel, error channel, and cancel function.
func (p *PipelineFunctionalSuite) WithRunningPipeline(
	testFunc func(pipeline Pipeline, updateChan chan State, errChan chan error, cancel context.CancelFunc),
	pipelineConfig PipelineConfig,
) {

	p.core = NewCoreImpl(
		p.logger,
		p.executionResult,
		p.block.ToHeader(),
		p.execDataRequester,
		p.txResultErrMsgsRequester,
		p.txResultErrMsgsRequestTimeout,
		p.persistentRegisters,
		p.persistentEvents,
		p.persistentCollections,
		p.persistentResults,
		p.persistentTxResultErrMsg,
		p.persistentLatestSealedResult,
		p.db,
		p.lockManager,
	)

	pipelineStateConsumer := NewMockStateConsumer()

	pipeline := NewPipeline(p.logger, p.executionResult, false, pipelineStateConsumer)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error)
	// wait until a pipeline goroutine run a pipeline
	pipelineIsReady := make(chan struct{})

	go func() {
		if pipelineConfig.beforePipelineRun != nil {
			pipelineConfig.beforePipelineRun(pipeline)
		}

		close(pipelineIsReady)

		errChan <- pipeline.Run(ctx, p.core, pipelineConfig.parentState)
	}()

	<-pipelineIsReady

	testFunc(pipeline, pipelineStateConsumer.updateChan, errChan, cancel)
}

// createExecutionData creates and returns test execution data and transaction result
// error messages for use in test cases. It generates realistic test data including
// chunk execution data with events, trie updates, collections, and system chunks.
func (p *PipelineFunctionalSuite) createExecutionData() (*execution_data.BlockExecutionData, []flow.TransactionResultErrorMessage) {
	expectedChunkExecutionData := unittest.ChunkExecutionDataFixture(
		p.T(),
		0,
		unittest.WithChunkEvents(unittest.EventsFixture(5)),
		unittest.WithTrieUpdate(indexer.TrieUpdateRandomLedgerPayloadsFixture(p.T())),
	)
	systemChunkCollection := unittest.CollectionFixture(1)
	systemChunkData := &execution_data.ChunkExecutionData{
		Collection: &systemChunkCollection,
	}

	expectedExecutionData := unittest.BlockExecutionDataFixture(
		unittest.WithBlockExecutionDataBlockID(p.block.ID()),
		unittest.WithChunkExecutionDatas(expectedChunkExecutionData, systemChunkData),
	)
	expectedTxResultErrMsgs := unittest.TransactionResultErrorMessagesFixture(5)
	return expectedExecutionData, expectedTxResultErrMsgs
}

// verifyDataPersistence checks that all expected data was actually persisted to storage.
// It verifies the persistence of events, collections, transaction results, registers,
// and transaction result error messages by comparing stored data with expected values.
func (p *PipelineFunctionalSuite) verifyDataPersistence(
	expectedChunkExecutionData *execution_data.ChunkExecutionData,
	expectedTxResultErrMsgs []flow.TransactionResultErrorMessage,
) {
	p.verifyEventsPersisted(expectedChunkExecutionData.Events)

	p.verifyCollectionPersisted(expectedChunkExecutionData.Collection)

	p.verifyTransactionResultsPersisted(expectedChunkExecutionData.TransactionResults)

	p.verifyRegistersPersisted(expectedChunkExecutionData.TrieUpdate)

	p.verifyTxResultErrorMessagesPersisted(expectedTxResultErrMsgs)
}

// verifyEventsPersisted checks that events were stored correctly in the events storage.
// It retrieves events by block ID and compares them with the expected events list.
func (p *PipelineFunctionalSuite) verifyEventsPersisted(expectedEvents flow.EventsList) {
	storedEvents, err := p.persistentEvents.ByBlockID(p.block.ID())
	p.Require().NoError(err)

	p.Assert().Equal(expectedEvents, flow.EventsList(storedEvents))
}

// verifyCollectionPersisted checks that the collection was stored correctly in the
// collections storage. It verifies both the light collection data and its transaction
// IDs are persisted correctly.
func (p *PipelineFunctionalSuite) verifyCollectionPersisted(expectedCollection *flow.Collection) {
	collectionID := expectedCollection.ID()
	expectedLightCollection := expectedCollection.Light()

	storedLightCollection, err := p.persistentCollections.LightByID(collectionID)
	p.Require().NoError(err)

	p.Assert().Equal(expectedLightCollection, storedLightCollection)
	p.Assert().ElementsMatch(expectedCollection.Light().Transactions, storedLightCollection.Transactions)
}

// verifyTransactionResultsPersisted checks that transaction results were stored correctly
// in the results storage. It retrieves results by block ID and compares them with expected results.
func (p *PipelineFunctionalSuite) verifyTransactionResultsPersisted(expectedResults []flow.LightTransactionResult) {
	storedResults, err := p.persistentResults.ByBlockID(p.block.ID())
	p.Require().NoError(err)

	p.Assert().ElementsMatch(expectedResults, storedResults)
}

// verifyRegistersPersisted checks that registers were stored correctly in the registers storage.
// It iterates through all payloads in the trie update and verifies each register value
// can be retrieved at the correct block height.
func (p *PipelineFunctionalSuite) verifyRegistersPersisted(expectedTrieUpdate *ledger.TrieUpdate) {
	for _, payload := range expectedTrieUpdate.Payloads {
		key, err := payload.Key()
		p.Require().NoError(err)

		registerID, err := convert.LedgerKeyToRegisterID(key)
		p.Require().NoError(err)

		storedValue, err := p.persistentRegisters.Get(registerID, p.block.Height)
		p.Require().NoError(err)

		expectedValue := payload.Value()
		p.Assert().Equal(expectedValue, ledger.Value(storedValue))
	}
}

// verifyTxResultErrorMessagesPersisted checks that transaction result error messages
// were stored correctly in the error messages storage. It retrieves messages by block ID
// and compares them with expected error messages.
func (p *PipelineFunctionalSuite) verifyTxResultErrorMessagesPersisted(
	expectedTxResultErrMsgs []flow.TransactionResultErrorMessage,
) {
	storedErrMsgs, err := p.persistentTxResultErrMsg.ByBlockID(p.block.ID())
	p.Require().NoError(err, "Should be able to retrieve tx result error messages by block ID")

	p.Assert().ElementsMatch(expectedTxResultErrMsgs, storedErrMsgs)
}
