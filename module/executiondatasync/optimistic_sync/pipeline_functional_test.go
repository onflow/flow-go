package optimistic_sync

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/onflow/flow-go/module/state_synchronization/indexer"

	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	txerrmsgsmock "github.com/onflow/flow-go/engine/access/ingestion/tx_error_messages/mock"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/metrics"
	reqestermock "github.com/onflow/flow-go/module/state_synchronization/requester/mock"
	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/operation/badgerimpl"
	pebbleStorage "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

type PipelineFunctionalSuite struct {
	suite.Suite
	logger                        zerolog.Logger
	execDataRequester             *reqestermock.ExecutionDataRequester
	txResultErrMsgsRequester      *txerrmsgsmock.TransactionResultErrorMessageRequester
	txResultErrMsgsRequestTimeout time.Duration
	tmpDir                        string
	bdb                           *badger.DB
	pdb                           *pebble.DB
	persistentRegisters           *pebbleStorage.Registers
	persistentEvents              *store.Events
	persistentCollections         *store.Collections
	persistentTransactions        *store.Transactions
	persistentResults             *store.LightTransactionResults
	persistentTxResultErrMsg      *store.TransactionResultErrorMessages
	core                          *CoreImpl
	block                         *flow.Block
	executionResult               *flow.ExecutionResult
	metrics                       module.CacheMetrics
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

	p.tmpDir = unittest.TempDir(t)
	p.logger = zerolog.Nop()
	p.metrics = metrics.NewNoopCollector()
	p.bdb = unittest.BadgerDB(t, p.tmpDir)
	db := badgerimpl.ToDB(p.bdb)

	rootBlock := unittest.BlockHeaderFixture()

	// Create real storages
	var err error
	p.pdb = pebbleStorage.NewBootstrappedRegistersWithPathForTest(t, p.tmpDir, rootBlock.Height, rootBlock.Height)
	p.persistentRegisters, err = pebbleStorage.NewRegisters(p.pdb, pebbleStorage.PruningDisabled)
	p.Require().NoError(err)

	p.persistentEvents = store.NewEvents(p.metrics, db)
	p.persistentTransactions = store.NewTransactions(p.metrics, db)
	p.persistentCollections = store.NewCollections(db, p.persistentTransactions)
	p.persistentResults = store.NewLightTransactionResults(p.metrics, db, bstorage.DefaultCacheSize)
	p.persistentTxResultErrMsg = store.NewTransactionResultErrorMessages(p.metrics, db, bstorage.DefaultCacheSize)

	p.block = unittest.BlockWithParentFixture(rootBlock)
	p.executionResult = unittest.ExecutionResultFixture(unittest.WithBlock(p.block))

	p.execDataRequester = reqestermock.NewExecutionDataRequester(t)
	p.txResultErrMsgsRequester = txerrmsgsmock.NewTransactionResultErrorMessageRequester(t)
	p.txResultErrMsgsRequestTimeout = DefaultTxResultErrMsgsRequestTimeout

	p.core = NewCoreImpl(
		p.logger,
		p.executionResult,
		p.block.Header,
		p.execDataRequester,
		p.txResultErrMsgsRequester,
		p.txResultErrMsgsRequestTimeout,
		p.persistentRegisters,
		p.persistentEvents,
		p.persistentCollections,
		p.persistentTransactions,
		p.persistentResults,
		p.persistentTxResultErrMsg,
		db,
	)
}

// TearDownTest cleans up resources after each test case.
// It closes database connections and removes temporary directories
// to ensure a clean state for subsequent tests.
func (p *PipelineFunctionalSuite) TearDownTest() {
	p.Require().NoError(p.pdb.Close())
	p.Require().NoError(p.bdb.Close())
	p.Require().NoError(os.RemoveAll(p.tmpDir))
}

// TestPipelineHappyCase verifies the complete happy path flow of the pipeline.
// It tests that:
// 1. Pipeline processes execution data through all states correctly
// 2. All data types (events, collections, transactions, registers, error messages) are correctly persisted to storage
// 3. No errors occur during the entire process
func (p *PipelineFunctionalSuite) TestPipelineHappyCase() {
	expectedExecutionData, expectedTxResultErrMsgs := p.initializeTestData()

	p.execDataRequester.On("RequestExecutionData", mock.Anything).Return(expectedExecutionData, nil).Once()
	p.txResultErrMsgsRequester.On("Request", mock.Anything).Return(expectedTxResultErrMsgs, nil).Once()

	p.WithRunningPipeline(func(pipeline Pipeline, updateChan chan State, errChan chan error, cancel context.CancelFunc) {
		pipeline.OnParentStateUpdated(StateComplete)

	mainLoop:
		for {
			select {
			case err := <-errChan:
				p.Require().NoError(err)
				return
			case newState := <-updateChan:
				p.Require().NotEqual(StateAbandoned, newState)

				switch newState {
				case StateWaitingPersist:
					pipeline.SetSealed()
				case StateComplete:
					break mainLoop
				default:
					continue
				}
			}
		}

		expectedChunkExecutionData := expectedExecutionData.ChunkExecutionDatas[0]
		p.verifyDataPersistence(expectedChunkExecutionData, expectedTxResultErrMsgs)
	})
}

// TestPipelineDownloadError tests error handling during the download phase.
// It verifies that when execution data request fails, the pipeline properly
// propagates the error.
func (p *PipelineFunctionalSuite) TestPipelineDownloadError() {
	// Test execution data request failure
	p.execDataRequester.On("RequestExecutionData", mock.Anything).Return((*execution_data.BlockExecutionData)(nil), assert.AnError).Once()
	p.txResultErrMsgsRequester.On("Request", mock.Anything).Return(([]flow.TransactionResultErrorMessage)(nil), nil).Maybe()

	p.WithRunningPipeline(func(pipeline Pipeline, updateChan chan State, errChan chan error, cancel context.CancelFunc) {
		pipeline.OnParentStateUpdated(StateComplete)

		err := <-errChan
		p.Require().ErrorIs(err, assert.AnError)
		p.Assert().Contains(err.Error(), "failed to request execution data")
	})
}

// TestPipelineIndexingError tests error handling during the indexing phase.
// It verifies that when execution data contains invalid block IDs, the pipeline
// properly detects the inconsistency and returns an appropriate error.
func (p *PipelineFunctionalSuite) TestPipelineIndexingError() {
	// Setup successful download
	expectedExecutionData := unittest.BlockExecutionDataFixture(
		unittest.WithBlockExecutionDataBlockID(unittest.IdentifierFixture()), // Wrong block ID to cause indexing error
	)
	p.execDataRequester.On("RequestExecutionData", mock.Anything).Return(expectedExecutionData, nil).Once()

	expectedTxResultErrMsgs := unittest.TransactionResultErrorMessagesFixture(5)
	p.txResultErrMsgsRequester.On("Request", mock.Anything).Return(expectedTxResultErrMsgs, nil).Once()

	p.WithRunningPipeline(func(pipeline Pipeline, updateChan chan State, errChan chan error, cancel context.CancelFunc) {
		pipeline.OnParentStateUpdated(StateComplete)

		// Should get error from pipeline during indexing
		err := <-errChan
		p.Require().Error(err)
		p.Assert().Contains(err.Error(), "invalid block execution data")
	})
}

// TestPipelinePersistError tests error handling during the persist phase.
// It simulates a persistence failure by corrupting the storage state
// and verifies that the pipeline properly handles and reports the error.
func (p *PipelineFunctionalSuite) TestPipelinePersistError() {
	expectedExecutionData, expectedTxResultErrMsgs := p.initializeTestData()

	p.execDataRequester.On("RequestExecutionData", mock.Anything).Return(expectedExecutionData, nil).Once()
	p.txResultErrMsgsRequester.On("Request", mock.Anything).Return(expectedTxResultErrMsgs, nil).Once()

	p.WithRunningPipeline(func(pipeline Pipeline, updateChan chan State, errChan chan error, cancel context.CancelFunc) {
		pipeline.OnParentStateUpdated(StateComplete)

	mainLoop:
		for {
			select {
			case err := <-errChan:
				p.Require().Error(err)
				p.Assert().Contains(err.Error(), "could not get events")
				return
			case newState := <-updateChan:
				switch newState {
				case StateWaitingPersist:
					err := p.persistentEvents.RemoveByBlockID(p.block.ID())
					p.Require().NoError(err)
					pipeline.SetSealed()
				case StateComplete:
					break mainLoop
				default:
					continue
				}
			}
		}
	})
}

// TestPipelineParentCtxCancellationDuringDownload tests context cancellation during the download phase.
// It verifies that when the parent context is cancelled while downloading execution data,
// the pipeline properly handles the cancellation and transitions to the appropriate state.
func (p *PipelineFunctionalSuite) TestPipelineParentCtxCancellationDuringDownload() {
	p.execDataRequester.On("RequestExecutionData", mock.Anything).Run(func(args mock.Arguments) {
		ctx := args.Get(0).(context.Context)
		// Wait for cancellation
		<-ctx.Done()
	}).Return((*execution_data.BlockExecutionData)(nil), context.Canceled).Once()

	p.txResultErrMsgsRequester.On("Request", mock.Anything).Return([]flow.TransactionResultErrorMessage{}, nil).Once()

	p.WithRunningPipeline(func(pipeline Pipeline, updateChan chan State, errChan chan error, cancel context.CancelFunc) {
		pipeline.OnParentStateUpdated(StateComplete)

		go func() {
			for state := range updateChan {
				if state == StateDownloading {
					// Cancel context during download
					cancel()
					return
				}
			}
		}()

		err := <-errChan
		p.Require().Error(err)

		p.Assert().ErrorIs(err, context.Canceled)
		p.Assert().Equal(StateDownloading, pipeline.GetState())
	})
}

// TestPipelineShutdownOnParentAbandon tests pipeline behavior when the parent state
// is abandoned. It verifies that the pipeline properly shuts down, cancels ongoing
// operations, and cleans up its working data when the parent abandons the operation.
func (p *PipelineFunctionalSuite) TestPipelineShutdownOnParentAbandon() {
	expectedExecutionData, expectedTxResultErrMsgs := p.initializeTestData()

	p.execDataRequester.On("RequestExecutionData", mock.Anything).Return(expectedExecutionData, nil).Once()
	p.txResultErrMsgsRequester.On("Request", mock.Anything).Return(expectedTxResultErrMsgs, nil).Once()

	p.WithRunningPipeline(func(pipeline Pipeline, updateChan chan State, errChan chan error, cancel context.CancelFunc) {

		pipeline.OnParentStateUpdated(StateWaitingPersist)

		go func() {
			for state := range updateChan {
				if state == StateWaitingPersist {
					pipeline.OnParentStateUpdated(StateAbandoned)
					return
				}
			}
		}()

		err := <-errChan
		p.Require().Error(err)

		p.Assert().ErrorIs(err, context.Canceled)
		p.Assert().Equal(StateAbandoned, pipeline.GetState())
		p.Assert().Nil(p.core.workingData)
	})
}

// WithRunningPipeline is a test helper that sets up and runs a pipeline instance
// with proper channel communication and context management. It provides the test
// function with access to the running pipeline, state update channel, error channel,
// and cancellation function for comprehensive testing scenarios.
func (p *PipelineFunctionalSuite) WithRunningPipeline(testFunc func(pipeline Pipeline, updateChan chan State, errChan chan error, cancel context.CancelFunc)) {
	updateChan := make(chan State, 10)

	publisher := func(state State) {
		updateChan <- state
	}

	pipeline := NewPipeline(p.logger, false, p.executionResult, p.core, publisher)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error)
	go func() {
		errChan <- pipeline.Run(ctx, p.core)
	}()

	testFunc(pipeline, updateChan, errChan, cancel)
}

// initializeTestData creates and returns test execution data and transaction result
// error messages for use in test cases. It generates realistic test data including
// chunk execution data with events, trie updates, collections, and system chunks.
func (p *PipelineFunctionalSuite) initializeTestData() (*execution_data.BlockExecutionData, []flow.TransactionResultErrorMessage) {
	expectedChunkExecutionData := unittest.ChunkExecutionDataFixture(
		p.T(),
		0,
		unittest.WithChunkEvents(unittest.EventsFixture(5)),
		unittest.WithTrieUpdate(indexer.CreateTestTrieUpdate(p.T())),
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

// verifyCollectionPersisted checks that the collection was stored correctly in the collections storage.
// It verifies both the light collection data and transaction IDs are properly persisted.
func (p *PipelineFunctionalSuite) verifyCollectionPersisted(expectedCollection *flow.Collection) {
	collectionID := expectedCollection.ID()
	expectedLightCollection := expectedCollection.Light()

	storedLightCollection, err := p.persistentCollections.LightByID(collectionID)
	p.Require().NoError(err)

	p.Assert().Equal(&expectedLightCollection, storedLightCollection)
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

		storedValue, err := p.persistentRegisters.Get(registerID, p.block.Header.Height)
		p.Require().NoError(err)

		expectedValue := payload.Value()
		p.Assert().Equal(expectedValue, ledger.Value(storedValue))
	}
}

// verifyTxResultErrorMessagesPersisted checks that transaction result error messages
// were stored correctly in the error messages storage. It retrieves messages by block ID
// and compares them with expected error messages.
func (p *PipelineFunctionalSuite) verifyTxResultErrorMessagesPersisted(expectedTxResultErrMsgs []flow.TransactionResultErrorMessage) {
	storedErrMsgs, err := p.persistentTxResultErrMsg.ByBlockID(p.block.ID())
	p.Require().NoError(err, "Should be able to retrieve tx result error messages by block ID")

	p.Assert().ElementsMatch(expectedTxResultErrMsgs, storedErrMsgs)
}
