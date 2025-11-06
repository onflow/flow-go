package optimistic_sync

import (
	"context"
	"fmt"
	"os"
	"testing"
	"testing/synctest"
	"time"

	"github.com/jordanschalm/lockctx"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	txerrmsgsmock "github.com/onflow/flow-go/engine/access/ingestion/tx_error_messages/mock"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/metrics"
	reqestermock "github.com/onflow/flow-go/module/state_synchronization/requester/mock"
	"github.com/onflow/flow-go/storage"
	bstorage "github.com/onflow/flow-go/storage/badger"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	pebbleStorage "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/fixtures"
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

	expectedData *expectedData
}

type expectedData struct {
	events          flow.EventsList
	results         []flow.LightTransactionResult
	collections     []*flow.Collection
	transactions    []*flow.TransactionBody
	registerEntries []flow.RegisterEntry
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

	g := fixtures.NewGeneratorSuite()

	rootBlock := g.Blocks().Fixture()
	sealedBlock := g.Blocks().Fixture(g.Blocks().WithParentHeader(rootBlock.ToHeader()))
	sealedExecutionResult := g.ExecutionResults().Fixture(g.ExecutionResults().WithBlock(sealedBlock))

	tf := generateFixtureExtendingLatestPersisted(g, sealedBlock.ToHeader(), sealedExecutionResult.ID())
	p.block = tf.block
	p.executionResult = tf.exeResult
	p.expectedExecutionData = tf.execData
	p.expectedTxResultErrMsgs = tf.txErrMsgs

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

	err = unittest.WithLock(t, p.lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
		return p.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.InsertHeader(lctx, rw, rootBlock.ID(), rootBlock.ToHeader())
		})
	})
	require.NoError(t, err)

	err = unittest.WithLock(t, p.lockManager, storage.LockFinalizeBlock, func(lctx lockctx.Context) error {
		return p.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.IndexFinalizedBlockByHeight(lctx, rw, rootBlock.Height, rootBlock.ID())
		})
	})
	require.NoError(t, err)

	// store and index the latest sealed block header
	err = unittest.WithLock(t, p.lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
		return p.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.InsertHeader(lctx, rw, sealedBlock.ID(), sealedBlock.ToHeader())
		})
	})
	require.NoError(t, err)

	err = unittest.WithLock(t, p.lockManager, storage.LockFinalizeBlock, func(lctx lockctx.Context) error {
		return p.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.IndexFinalizedBlockByHeight(lctx, rw, sealedBlock.Height, sealedBlock.ID())
		})
	})
	require.NoError(t, err)

	// Store and index sealed block execution result
	err = unittest.WithLock(t, p.lockManager, storage.LockIndexExecutionResult, func(lctx lockctx.Context) error {
		return p.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			err := p.results.BatchStore(sealedExecutionResult, rw)
			p.Require().NoError(err)

			err = p.results.BatchIndex(lctx, rw, sealedBlock.ID(), sealedExecutionResult.ID())
			p.Require().NoError(err)
			return nil
		})
	})
	p.Require().NoError(err)

	p.persistentLatestSealedResult, err = store.NewLatestPersistedSealedResult(p.consumerProgress, p.headers, p.results)
	p.Require().NoError(err)

	p.execDataRequester = reqestermock.NewExecutionDataRequester(t)
	p.txResultErrMsgsRequester = txerrmsgsmock.NewRequester(t)
	p.txResultErrMsgsRequestTimeout = DefaultTxResultErrMsgsRequestTimeout

	p.config = PipelineConfig{
		parentState: StateWaitingPersist,
	}

	//  generate expected data based on the fixtures
	var expectedEvents flow.EventsList
	var expectedResults []flow.LightTransactionResult
	var expectedRegisterEntries []flow.RegisterEntry
	var expectedCollections []*flow.Collection
	var expectedTransactions []*flow.TransactionBody
	for i, chunk := range p.expectedExecutionData.ChunkExecutionDatas {
		expectedEvents = append(expectedEvents, chunk.Events...)
		expectedResults = append(expectedResults, chunk.TransactionResults...)

		// skip the system chunk collection
		if i < len(p.expectedExecutionData.ChunkExecutionDatas)-1 {
			expectedCollections = append(expectedCollections, chunk.Collection)
			expectedTransactions = append(expectedTransactions, chunk.Collection.Transactions...)
		}

		for j, payload := range chunk.TrieUpdate.Payloads {
			key, value, err := convert.PayloadToRegister(payload)
			p.Require().NoError(err)

			if j == 0 && i < len(p.expectedExecutionData.ChunkExecutionDatas)-1 {
				continue
			}

			expectedRegisterEntries = append(expectedRegisterEntries, flow.RegisterEntry{
				Key:   key,
				Value: value,
			})
		}
	}

	p.expectedData = &expectedData{
		events:          expectedEvents,
		results:         expectedResults,
		collections:     expectedCollections,
		transactions:    expectedTransactions,
		registerEntries: expectedRegisterEntries,
	}
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
		// Check for errors in a separate goroutine
		go func() {
			err := <-errChan
			if err != nil {
				p.T().Errorf("Pipeline error: %v", err)
			}
		}()

		pipeline.OnParentStateUpdated(StateComplete)

		waitForStateUpdates(p.T(), updateChan, errChan, StateProcessing, StateWaitingPersist)

		pipeline.SetSealed()

		waitForStateUpdates(p.T(), updateChan, errChan, StateComplete)

		actualEvents, err := p.persistentEvents.ByBlockID(p.block.ID())
		p.Require().NoError(err)
		p.Assert().Equal([]flow.Event(p.expectedData.events), actualEvents)

		actualResults, err := p.persistentResults.ByBlockID(p.block.ID())
		p.Require().NoError(err)
		p.Assert().Equal(p.expectedData.results, actualResults)

		for _, expectedCollection := range p.expectedData.collections {
			actualCollection, err := p.persistentCollections.ByID(expectedCollection.ID())
			p.Require().NoError(err)
			p.Assert().Equal(expectedCollection, actualCollection)

			actualLightCollection, err := p.persistentCollections.LightByID(expectedCollection.ID())
			p.Require().NoError(err)
			p.Assert().Equal(expectedCollection.Light(), actualLightCollection)
		}

		for _, expectedTransaction := range p.expectedData.transactions {
			actualTransaction, err := p.persistentTransactions.ByID(expectedTransaction.ID())
			p.Require().NoError(err)
			p.Assert().Equal(expectedTransaction, actualTransaction)
		}

		for _, expectedRegisterEntry := range p.expectedData.registerEntries {
			actualValue, err := p.persistentRegisters.Get(expectedRegisterEntry.Key, p.block.Height)
			p.Require().NoError(err)
			p.Assert().Equal(expectedRegisterEntry.Value, actualValue)
		}

		actualTxResultErrMsgs, err := p.persistentTxResultErrMsg.ByBlockID(p.block.ID())
		p.Require().NoError(err)
		p.Assert().Equal(p.expectedTxResultErrMsgs, actualTxResultErrMsgs)

	}, p.config)
}

// TestPipelineDownloadError tests how the pipeline handles errors during the download phase.
// It ensures that both execution data and transaction result error message request errors
// are correctly detected and returned.
func (p *PipelineFunctionalSuite) TestPipelineDownloadError() {
	p.Run("execution data requester malformed data error", func() {
		expectedErr := execution_data.NewMalformedDataError(fmt.Errorf("execution data test deserialization error"))

		p.execDataRequester.On("RequestExecutionData", mock.Anything).Return(nil, expectedErr).Once()
		p.txResultErrMsgsRequester.On("Request", mock.Anything).Return(p.expectedTxResultErrMsgs, nil).Once()

		p.WithRunningPipeline(func(pipeline Pipeline, updateChan chan State, errChan chan error, cancel context.CancelFunc) {
			pipeline.OnParentStateUpdated(StateComplete)

			waitForError(p.T(), errChan, expectedErr)
			p.Assert().Equal(StateProcessing, pipeline.GetState())
		}, p.config)
	})

	p.Run("transaction result error messages requester not found error", func() {
		expectedErr := fmt.Errorf("test transaction result error messages not found error")

		p.execDataRequester.On("RequestExecutionData", mock.Anything).Return(p.expectedExecutionData, nil).Once()
		p.txResultErrMsgsRequester.On("Request", mock.Anything).Return(nil, expectedErr).Once()

		p.WithRunningPipeline(func(pipeline Pipeline, updateChan chan State, errChan chan error, cancel context.CancelFunc) {
			pipeline.OnParentStateUpdated(StateComplete)

			waitForError(p.T(), errChan, expectedErr)
			p.Assert().Equal(StateProcessing, pipeline.GetState())
		}, p.config)
	})
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

	expectedIndexingError := fmt.Errorf("could not perform indexing: failed to index execution data: unexpected block execution data: expected block_id=%s, actual block_id=%s",
		p.block.ID().String(), invalidBlockID.String())

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
	mockEvents.On("BatchStore",
		mock.MatchedBy(func(lctx lockctx.Proof) bool { return lctx.HoldsLock(storage.LockInsertEvent) }),
		p.block.ID(), mock.Anything, mock.Anything).Return(expectedError).Once()
	p.persistentEvents = mockEvents

	p.execDataRequester.On("RequestExecutionData", mock.Anything).Return(p.expectedExecutionData, nil).Once()
	p.txResultErrMsgsRequester.On("Request", mock.Anything).Return(p.expectedTxResultErrMsgs, nil).Once()

	p.WithRunningPipeline(func(pipeline Pipeline, updateChan chan State, errChan chan error, cancel context.CancelFunc) {
		pipeline.OnParentStateUpdated(StateComplete)

		waitForStateUpdates(p.T(), updateChan, errChan, StateProcessing, StateWaitingPersist)

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
		p.execDataRequester.
			On("RequestExecutionData", mock.Anything).
			Return(func(ctx context.Context) (*execution_data.BlockExecutionData, error) {
				cancel()
				<-ctx.Done()
				return nil, ctx.Err()
			}).
			Once()

		// This call marked as `Maybe()` because it may not be called depending on timing.
		p.txResultErrMsgsRequester.On("Request", mock.Anything).Return([]flow.TransactionResultErrorMessage{}, nil).Maybe()

		pipeline.OnParentStateUpdated(StateComplete)

		waitForStateUpdates(p.T(), updateChan, errChan, StateProcessing)
		waitForError(p.T(), errChan, context.Canceled)

		p.Assert().Equal(StateProcessing, pipeline.GetState())
	}, PipelineConfig{parentState: StatePending})
}

// TestMainCtxCancellationDuringRequestingTxResultErrMsgs tests context cancellation during
// the request of transaction result error messages. It verifies that when the parent context
// is cancelled during this phase, the pipeline handles the cancellation gracefully
// and transitions to the correct state.
func (p *PipelineFunctionalSuite) TestMainCtxCancellationDuringRequestingTxResultErrMsgs() {
	p.WithRunningPipeline(func(pipeline Pipeline, updateChan chan State, errChan chan error, cancel context.CancelFunc) {
		// This call marked as `Maybe()` because it may not be called depending on timing.
		p.execDataRequester.On("RequestExecutionData", mock.Anything).Return(nil, nil).Maybe()

		p.txResultErrMsgsRequester.
			On("Request", mock.Anything).
			Return(func(ctx context.Context) ([]flow.TransactionResultErrorMessage, error) {
				cancel()
				<-ctx.Done()
				return nil, ctx.Err()
			}).
			Once()

		pipeline.OnParentStateUpdated(StateComplete)

		waitForStateUpdates(p.T(), updateChan, errChan, StateProcessing)
		waitForError(p.T(), errChan, context.Canceled)

		p.Assert().Equal(StateProcessing, pipeline.GetState())
	}, PipelineConfig{parentState: StatePending})
}

// TestMainCtxCancellationDuringWaitingPersist tests the pipeline's behavior when the main context is canceled during StateWaitingPersist.
func (p *PipelineFunctionalSuite) TestMainCtxCancellationDuringWaitingPersist() {
	p.execDataRequester.On("RequestExecutionData", mock.Anything).Return(p.expectedExecutionData, nil).Once()
	p.txResultErrMsgsRequester.On("Request", mock.Anything).Return(p.expectedTxResultErrMsgs, nil).Once()

	p.WithRunningPipeline(func(pipeline Pipeline, updateChan chan State, errChan chan error, cancel context.CancelFunc) {
		pipeline.OnParentStateUpdated(StateComplete)

		waitForStateUpdates(p.T(), updateChan, errChan, StateProcessing, StateWaitingPersist)

		cancel()

		pipeline.SetSealed()

		waitForError(p.T(), errChan, context.Canceled)

		p.Assert().Equal(StateWaitingPersist, pipeline.GetState())
	}, p.config)
}

// TestPipelineShutdownOnParentAbandon verifies that the pipeline transitions correctly to a shutdown state when the parent is abandoned.
func (p *PipelineFunctionalSuite) TestPipelineShutdownOnParentAbandon() {
	assertNoError := func(err error) {
		p.Require().NoError(err)
	}

	tests := []struct {
		name        string
		config      PipelineConfig
		checkError  func(err error)
		customSetup func(pipeline Pipeline, updateChan chan State, errChan chan error)
	}{
		{
			name: "from StatePending",
			config: PipelineConfig{
				beforePipelineRun: func(pipeline *PipelineImpl) {
					pipeline.OnParentStateUpdated(StateAbandoned)
				},
				parentState: StateAbandoned,
			},
			checkError:  assertNoError,
			customSetup: func(pipeline Pipeline, updateChan chan State, errChan chan error) {},
		},
		{
			name: "from StateProcessing",
			config: PipelineConfig{
				beforePipelineRun: func(pipeline *PipelineImpl) {
					p.execDataRequester.On("RequestExecutionData", mock.Anything).Return(func(ctx context.Context) (*execution_data.BlockExecutionData, error) {
						pipeline.OnParentStateUpdated(StateAbandoned) // abandon during processing step
						return p.expectedExecutionData, nil
					}).Once()
					// this method may not be called depending on how quickly the RequestExecutionData
					// mock returns.
					p.txResultErrMsgsRequester.On("Request", mock.Anything).Return(p.expectedTxResultErrMsgs, nil).Maybe()
				},
				parentState: StateWaitingPersist,
			},
			checkError: func(err error) {
				// depending on the timing, the error may be during or after the indexing step.
				if err != nil {
					p.Require().ErrorContains(err, "could not perform indexing")
				} else {
					p.Require().NoError(err)
				}
			},
			customSetup: func(pipeline Pipeline, updateChan chan State, errChan chan error) {
				synctestWaitForStateUpdates(p.T(), updateChan, StateProcessing)
			},
		},
		{
			name: "from StateWaitingPersist",
			config: PipelineConfig{
				beforePipelineRun: func(pipeline *PipelineImpl) {
					p.execDataRequester.On("RequestExecutionData", mock.Anything).Return(p.expectedExecutionData, nil).Once()
					p.txResultErrMsgsRequester.On("Request", mock.Anything).Return(p.expectedTxResultErrMsgs, nil).Once()
				},
				parentState: StateWaitingPersist,
			},
			checkError: assertNoError,
			customSetup: func(pipeline Pipeline, updateChan chan State, errChan chan error) {
				synctestWaitForStateUpdates(p.T(), updateChan, StateProcessing, StateWaitingPersist)
				pipeline.OnParentStateUpdated(StateAbandoned)
			},
		},
	}

	for _, test := range tests {
		p.T().Run(test.name, func(t *testing.T) {
			p.execDataRequester.On("RequestExecutionData", mock.Anything).Unset()
			p.txResultErrMsgsRequester.On("Request", mock.Anything).Unset()

			synctest.Test(p.T(), func(t *testing.T) {
				p.WithRunningPipeline(func(pipeline Pipeline, updateChan chan State, errChan chan error, cancel context.CancelFunc) {
					test.customSetup(pipeline, updateChan, errChan)

					synctestWaitForStateUpdates(p.T(), updateChan, StateAbandoned)
					test.checkError(<-errChan)

					p.Assert().Equal(StateAbandoned, pipeline.GetState())
					p.Assert().Nil(p.core.workingData)
				}, test.config)
			})
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
	var err error

	p.core, err = NewCoreImpl(
		p.logger,
		p.executionResult,
		p.block,
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
	p.Require().NoError(err)

	pipelineStateConsumer := NewMockStateConsumer()

	pipeline := NewPipeline(p.logger, p.executionResult, false, pipelineStateConsumer)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error)
	// wait until a pipeline goroutine run a pipeline
	pipelineIsReady := make(chan struct{})

	go func() {
		defer close(errChan)

		if pipelineConfig.beforePipelineRun != nil {
			pipelineConfig.beforePipelineRun(pipeline)
		}

		close(pipelineIsReady)

		err := pipeline.Run(ctx, p.core, pipelineConfig.parentState)
		if err != nil {
			errChan <- err
		}
	}()

	<-pipelineIsReady

	testFunc(pipeline, pipelineStateConsumer.updateChan, errChan, cancel)
}

// generateFixture generates a test fixture for the indexer. The returned data has the following
// properties:
//   - The block execution data contains collections for each of the block's guarantees, plus the system chunk
//   - Each collection has 3 transactions
//   - The first path in each trie update is the same, testing that the indexer will use the last value
//   - Every 3rd transaction is failed
//   - There are tx error messages for all failed transactions
func generateFixtureExtendingLatestPersisted(g *fixtures.GeneratorSuite, parentHeader *flow.Header, latestPersistedResultID flow.Identifier) *testFixture {
	collections := g.Collections().List(4, fixtures.Collection.WithTxCount(3))
	chunkExecutionDatas := make([]*execution_data.ChunkExecutionData, len(collections))
	guarantees := make([]*flow.CollectionGuarantee, len(collections)-1)
	var txErrMsgs []flow.TransactionResultErrorMessage
	path := g.LedgerPaths().Fixture()
	for i, collection := range collections {
		chunkData := g.ChunkExecutionDatas().Fixture(
			fixtures.ChunkExecutionData.WithCollection(collection),
		)
		// use the same path fo the first ledger payload in each chunk. the indexer should chose the
		// last value in the register entry.
		chunkData.TrieUpdate.Paths[0] = path
		chunkExecutionDatas[i] = chunkData

		if i < len(collections)-1 {
			guarantees[i] = g.Guarantees().Fixture(fixtures.Guarantee.WithCollectionID(collection.ID()))
		}
		for txIndex := range chunkExecutionDatas[i].TransactionResults {
			if txIndex%3 == 0 {
				chunkExecutionDatas[i].TransactionResults[txIndex].Failed = true
			}
		}
		txErrMsgs = append(txErrMsgs, g.TransactionErrorMessages().ForTransactionResults(chunkExecutionDatas[i].TransactionResults)...)
	}

	payload := g.Payloads().Fixture(fixtures.Payload.WithGuarantees(guarantees...))
	block := g.Blocks().Fixture(
		fixtures.Block.WithParentHeader(parentHeader),
		fixtures.Block.WithPayload(payload),
	)

	exeResult := g.ExecutionResults().Fixture(
		fixtures.ExecutionResult.WithBlock(block),
		fixtures.ExecutionResult.WithPreviousResultID(latestPersistedResultID),
	)
	execData := g.BlockExecutionDatas().Fixture(
		fixtures.BlockExecutionData.WithBlockID(block.ID()),
		fixtures.BlockExecutionData.WithChunkExecutionDatas(chunkExecutionDatas...),
	)
	return &testFixture{
		block:     block,
		exeResult: exeResult,
		execData:  execData,
		txErrMsgs: txErrMsgs,
	}
}
