package pipeline

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	txerrmsgsmock "github.com/onflow/flow-go/engine/access/ingestion/tx_error_messages/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	reqestermock "github.com/onflow/flow-go/module/state_synchronization/requester/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/fixtures"
)

// CoreImplSuite is a test suite for testing the CoreImpl.
type CoreImplSuite struct {
	suite.Suite
	execDataRequester             *reqestermock.ExecutionDataRequester
	txResultErrMsgsRequester      *txerrmsgsmock.Requester
	txResultErrMsgsRequestTimeout time.Duration
	db                            *storagemock.DB
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
	t := c.T()

	c.execDataRequester = reqestermock.NewExecutionDataRequester(t)
	c.txResultErrMsgsRequester = txerrmsgsmock.NewRequester(t)
	c.txResultErrMsgsRequestTimeout = 100 * time.Millisecond
}

// createTestCoreImpl creates a CoreImpl instance with mocked dependencies for testing.
//
// Returns a configured CoreImpl ready for testing.
func (c *CoreImplSuite) createTestCoreImpl(tf *testFixture) *CoreImpl {
	core, err := NewCoreImpl(
		unittest.Logger(),
		tf.exeResult,
		tf.block,
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
		storage.NewTestingLockManager(),
	)
	c.NoError(err)
	return core
}

type testFixture struct {
	block     *flow.Block
	exeResult *flow.ExecutionResult
	execData  *execution_data.BlockExecutionData
	txErrMsgs []flow.TransactionResultErrorMessage
}

// generateFixture generates a test fixture for the indexer. The returned data has the following
// properties:
//   - The block execution data contains collections for each of the block's guarantees, plus the system chunk
//   - Each collection has 3 transactions
//   - The first path in each trie update is the same, testing that the indexer will use the last value
//   - Every 3rd transaction is failed
//   - There are tx error messages for all failed transactions
func generateFixture(g *fixtures.GeneratorSuite) *testFixture {
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
	block := g.Blocks().Fixture(fixtures.Block.WithPayload(payload))

	exeResult := g.ExecutionResults().Fixture(fixtures.ExecutionResult.WithBlock(block))
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

func (c *CoreImplSuite) TestCoreImpl_Constructor() {
	block := unittest.BlockFixture()
	executionResult := unittest.ExecutionResultFixture(unittest.WithBlock(block))

	c.Run("happy path", func() {
		core, err := NewCoreImpl(
			unittest.Logger(),
			executionResult,
			block,
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
			storage.NewTestingLockManager(),
		)
		c.NoError(err)
		c.NotNil(core)
	})

	c.Run("block ID mismatch", func() {
		core, err := NewCoreImpl(
			unittest.Logger(),
			executionResult,
			unittest.BlockFixture(),
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
			storage.NewTestingLockManager(),
		)
		c.ErrorContains(err, "header ID and execution result block ID must match")
		c.Nil(core)
	})
}

// TestCoreImpl_Download tests the Download method retrieves execution data and transaction error
// messages.
func (c *CoreImplSuite) TestCoreImpl_Download() {
	ctx := context.Background()
	g := fixtures.NewGeneratorSuite()

	eventuallyCanceled := func(args mock.Arguments) {
		// make sure that the context is eventually canceled
		ctx := args.Get(0).(context.Context)
		c.Require().Eventually(func() bool {
			return ctx.Err() != nil
		}, time.Second, 10*time.Millisecond)
	}

	c.Run("successful download", func() {
		tf := generateFixture(g)
		core := c.createTestCoreImpl(tf)

		c.execDataRequester.On("RequestExecutionData", mock.Anything).Return(tf.execData, nil).Once()
		c.txResultErrMsgsRequester.On("Request", mock.Anything).Return(tf.txErrMsgs, nil).Once()

		err := core.Download(ctx)
		c.Require().NoError(err)

		c.Assert().Equal(tf.execData, core.workingData.executionData)
		c.Assert().Equal(tf.txErrMsgs, core.workingData.txResultErrMsgsData)

		// downloading a second time should return an error
		err = core.Download(ctx)
		c.ErrorContains(err, "already downloaded")
	})

	c.Run("execution data request error", func() {
		tf := generateFixture(g)
		core := c.createTestCoreImpl(tf)

		expectedErr := fmt.Errorf("test execution data request error")

		c.execDataRequester.On("RequestExecutionData", mock.Anything).Return(nil, expectedErr).Once()
		c.txResultErrMsgsRequester.
			On("Request", mock.Anything).
			Return(nil, context.Canceled).
			Run(eventuallyCanceled).Once()

		err := core.Download(ctx)
		c.Require().Error(err)

		c.Assert().ErrorIs(err, expectedErr)
		c.Assert().Nil(core.workingData.executionData)
		c.Assert().Nil(core.workingData.txResultErrMsgsData)
	})

	c.Run("transaction result error messages request error", func() {
		tf := generateFixture(g)
		core := c.createTestCoreImpl(tf)

		expectedErr := fmt.Errorf("test tx error messages request error")

		c.execDataRequester.
			On("RequestExecutionData", mock.Anything).
			Return(nil, context.Canceled).
			Run(eventuallyCanceled).Once()
		c.txResultErrMsgsRequester.On("Request", mock.Anything).Return(nil, expectedErr).Once()

		err := core.Download(ctx)
		c.Require().Error(err)

		c.Assert().ErrorIs(err, expectedErr)
		c.Assert().Nil(core.workingData.executionData)
		c.Assert().Nil(core.workingData.txResultErrMsgsData)
	})

	c.Run("context cancellation", func() {
		tf := generateFixture(g)
		core := c.createTestCoreImpl(tf)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		c.execDataRequester.
			On("RequestExecutionData", mock.Anything).
			Return(nil, ctx.Err()).
			Run(eventuallyCanceled).
			Once()
		c.txResultErrMsgsRequester.
			On("Request", mock.Anything).
			Return(nil, ctx.Err()).
			Run(eventuallyCanceled).
			Once()

		err := core.Download(ctx)
		c.Require().Error(err)

		c.Assert().ErrorIs(err, context.Canceled)
		c.Assert().Nil(core.workingData.executionData)
		c.Assert().Nil(core.workingData.txResultErrMsgsData)
	})

	c.Run("txResultErrMsgsRequestTimeout expiration", func() {
		tf := generateFixture(g)
		core := c.createTestCoreImpl(tf)

		c.execDataRequester.On("RequestExecutionData", mock.Anything).Return(tf.execData, nil).Once()

		// Transaction result error messages request times out
		c.txResultErrMsgsRequester.
			On("Request", mock.Anything).
			Return(nil, context.DeadlineExceeded).
			Run(func(args mock.Arguments) {
				// Simulate a slow request by sleeping longer than the timeout
				time.Sleep(2 * c.txResultErrMsgsRequestTimeout)
			}).
			Once()

		unittest.AssertReturnsBefore(c.T(), func() {
			err := core.Download(ctx)
			c.Require().NoError(err)
		}, time.Second)

		// the tx error messages timeout should be handled gracefully, and the execution data should
		// be downloaded and stored
		c.Assert().Equal(tf.execData, core.workingData.executionData)
		c.Assert().Nil(core.workingData.txResultErrMsgsData)
	})

	c.Run("Download after Abandon returns an error", func() {
		tf := generateFixture(g)
		core := c.createTestCoreImpl(tf)

		core.Abandon()
		c.Nil(core.workingData)

		err := core.Download(ctx)
		c.ErrorIs(err, optimistic_sync.ErrResultAbandoned)
	})
}

// TestCoreImpl_Index tests the Index method which processes downloaded data.
func (c *CoreImplSuite) TestCoreImpl_Index() {
	ctx := context.Background()
	g := fixtures.NewGeneratorSuite()

	tf := generateFixture(g)
	c.execDataRequester.On("RequestExecutionData", mock.Anything).Return(tf.execData, nil)
	c.txResultErrMsgsRequester.On("Request", mock.Anything).Return(tf.txErrMsgs, nil)

	c.Run("successful indexing", func() {
		core := c.createTestCoreImpl(tf)

		err := core.Download(ctx)
		c.Require().NoError(err)

		err = core.Index()
		c.Require().NoError(err)

		c.NotNil(core.workingData.indexerData)
		c.NotNil(core.workingData.inmemCollections)
		c.NotNil(core.workingData.inmemTransactions)
		c.NotNil(core.workingData.inmemTxResultErrMsgs)
		c.NotNil(core.workingData.inmemEvents)
		c.NotNil(core.workingData.inmemResults)
		c.NotNil(core.workingData.inmemRegisters)

		// indexing a second time should return an error
		err = core.Index()
		c.ErrorContains(err, "already indexed")
		c.NotNil(core.workingData.indexerData)
	})

	c.Run("indexer constructor error", func() {
		core := c.createTestCoreImpl(tf)
		core.block = g.Blocks().Fixture()

		err := core.Download(ctx)
		c.Require().NoError(err)

		err = core.Index()
		c.ErrorContains(err, "failed to create indexer")
		c.Nil(core.workingData.indexerData)
	})

	c.Run("failed to index block", func() {
		core := c.createTestCoreImpl(tf)

		err := core.Download(ctx)
		c.Require().NoError(err)

		core.workingData.executionData = g.BlockExecutionDatas().Fixture()

		err = core.Index()
		c.ErrorContains(err, "failed to index execution data")
		c.Nil(core.workingData.indexerData)
	})

	c.Run("failed to validate transaction result error messages", func() {
		core := c.createTestCoreImpl(tf)

		err := core.Download(ctx)
		c.Require().NoError(err)

		// add an extra error message
		core.workingData.txResultErrMsgsData = append(core.workingData.txResultErrMsgsData, flow.TransactionResultErrorMessage{
			TransactionID: g.Identifiers().Fixture(),
		})

		err = core.Index()
		c.ErrorContains(err, "failed to validate transaction result error messages")
		c.Nil(core.workingData.indexerData)
	})

	c.Run("Index after Abandon returns an error", func() {
		core := c.createTestCoreImpl(tf)

		core.Abandon()
		c.Nil(core.workingData)

		err := core.Index()
		c.ErrorIs(err, optimistic_sync.ErrResultAbandoned)
	})

	c.Run("Index before Download returns an error", func() {
		core := c.createTestCoreImpl(tf)

		err := core.Index()
		c.ErrorContains(err, "downloading is not complete")
		c.Nil(core.workingData.indexerData)
	})
}

// TestCoreImpl_Persist tests the Persist method which persists indexed data to storages and database.
func (c *CoreImplSuite) TestCoreImpl_Persist() {
	t := c.T()
	ctx := context.Background()
	g := fixtures.NewGeneratorSuite()

	resetMocks := func() {
		c.db = storagemock.NewDB(t)
		c.persistentRegisters = storagemock.NewRegisterIndex(t)
		c.persistentEvents = storagemock.NewEvents(t)
		c.persistentCollections = storagemock.NewCollections(t)
		c.persistentTransactions = storagemock.NewTransactions(t)
		c.persistentResults = storagemock.NewLightTransactionResults(t)
		c.persistentTxResultErrMsg = storagemock.NewTransactionResultErrorMessages(t)
		c.latestPersistedSealedResult = storagemock.NewLatestPersistedSealedResult(t)
	}

	tf := generateFixture(g)
	blockID := tf.block.ID()
	c.execDataRequester.On("RequestExecutionData", mock.Anything).Return(tf.execData, nil)
	c.txResultErrMsgsRequester.On("Request", mock.Anything).Return(tf.txErrMsgs, nil)

	c.Run("successful persistence of empty data", func() {
		resetMocks()
		core := c.createTestCoreImpl(tf)

		err := core.Download(ctx)
		c.Require().NoError(err)

		err = core.Index()
		c.Require().NoError(err)

		c.db.
			On("WithReaderBatchWriter", mock.Anything).
			Return(func(fn func(storage.ReaderBatchWriter) error) error {
				return fn(storagemock.NewBatch(t))
			}).
			Once()

		indexerData := core.workingData.indexerData
		c.persistentRegisters.On("Store", flow.RegisterEntries(indexerData.Registers), tf.block.Height).Return(nil)
		c.persistentEvents.On("BatchStore",
			mock.MatchedBy(func(lctx lockctx.Proof) bool { return lctx.HoldsLock(storage.LockInsertEvent) }),
			blockID, []flow.EventsList{indexerData.Events}, mock.MatchedBy(func(batch storage.ReaderBatchWriter) bool { return batch != nil })).Return(nil)
		for _, collection := range indexerData.Collections {
			c.persistentCollections.On("BatchStoreAndIndexByTransaction",
				mock.MatchedBy(func(lctx lockctx.Proof) bool {
					return lctx.HoldsLock(storage.LockInsertCollection)
				}),
				collection, mock.MatchedBy(func(batch storage.ReaderBatchWriter) bool { return batch != nil })).Return(nil, nil).Once()
		}
		c.persistentResults.On("BatchStore",
			mock.MatchedBy(func(lctx lockctx.Proof) bool {
				return lctx.HoldsLock(storage.LockInsertLightTransactionResult)
			}),
			mock.MatchedBy(func(batch storage.ReaderBatchWriter) bool { return batch != nil }),
			blockID, indexerData.Results).Return(nil)
		c.persistentTxResultErrMsg.On("BatchStore",
			mock.MatchedBy(func(lctx lockctx.Proof) bool {
				return lctx.HoldsLock(storage.LockInsertTransactionResultErrMessage)
			}),
			mock.MatchedBy(func(batch storage.ReaderBatchWriter) bool { return batch != nil }),
			blockID, core.workingData.txResultErrMsgsData).Return(nil)
		c.latestPersistedSealedResult.On("BatchSet", tf.exeResult.ID(), tf.block.Height,
			mock.MatchedBy(func(batch storage.ReaderBatchWriter) bool { return batch != nil })).Return(nil)

		err = core.Persist()
		c.Require().NoError(err)

		// persisting a second time should return an error
		err = core.Persist()
		c.ErrorContains(err, "already persisted")
	})

	c.Run("persisting registers fails", func() {
		expectedErr := fmt.Errorf("test persisting registers failure")

		resetMocks()
		core := c.createTestCoreImpl(tf)

		err := core.Download(ctx)
		c.Require().NoError(err)

		err = core.Index()
		c.Require().NoError(err)

		indexerData := core.workingData.indexerData
		c.persistentRegisters.On("Store", flow.RegisterEntries(indexerData.Registers), tf.block.Height).Return(expectedErr).Once()

		err = core.Persist()
		c.ErrorIs(err, expectedErr)
	})

	c.Run("persisting block data fails", func() {
		expectedErr := fmt.Errorf("test persisting events failure")

		resetMocks()
		core := c.createTestCoreImpl(tf)

		err := core.Download(ctx)
		c.Require().NoError(err)

		err = core.Index()
		c.Require().NoError(err)

		c.db.
			On("WithReaderBatchWriter", mock.Anything).
			Return(func(fn func(storage.ReaderBatchWriter) error) error {
				return fn(storagemock.NewBatch(t))
			}).
			Once()

		indexerData := core.workingData.indexerData
		c.persistentRegisters.On("Store", flow.RegisterEntries(indexerData.Registers), tf.block.Height).Return(nil).Once()
		c.persistentEvents.On("BatchStore",
			mock.MatchedBy(func(lctx lockctx.Proof) bool { return lctx.HoldsLock(storage.LockInsertEvent) }),
			blockID, []flow.EventsList{indexerData.Events}, mock.MatchedBy(func(batch storage.ReaderBatchWriter) bool { return batch != nil })).Return(expectedErr).Once()

		err = core.Persist()
		c.ErrorIs(err, expectedErr)
	})

	c.Run("Persist after Abandon returns an error", func() {
		resetMocks()
		core := c.createTestCoreImpl(tf)

		core.Abandon()
		c.Nil(core.workingData)

		err := core.Persist()
		c.ErrorIs(err, optimistic_sync.ErrResultAbandoned)
	})

	c.Run("Persist before Index returns an error", func() {
		resetMocks()
		core := c.createTestCoreImpl(tf)
		err := core.Persist()
		c.ErrorContains(err, "indexing is not complete")
	})
}
