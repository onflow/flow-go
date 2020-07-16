package finder_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/verification/finder"
	"github.com/dapperlabs/flow-go/engine/verification/utils"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/verification"
	realModule "github.com/dapperlabs/flow-go/module"
	mempool "github.com/dapperlabs/flow-go/module/mempool/mock"
	module "github.com/dapperlabs/flow-go/module/mock"
	"github.com/dapperlabs/flow-go/module/trace"
	network "github.com/dapperlabs/flow-go/network/mock"
	storage "github.com/dapperlabs/flow-go/storage/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// FinderEngineTestSuite contains the unit tests of Finder engine.
type FinderEngineTestSuite struct {
	suite.Suite
	net *module.Network
	me  *module.Local

	// mock conduit for receiving receipts
	receiptsConduit *network.Conduit
	metrics         *module.VerificationMetrics
	tracer          realModule.Tracer

	// mock mempools
	cachedReceipts     *mempool.ReceiptDataPacks
	pendingReceipts    *mempool.ReceiptDataPacks
	readyReceipts      *mempool.ReceiptDataPacks
	processedResultIDs *mempool.Identifiers
	blockIDsCache      *mempool.Identifiers
	receiptIDsByBlock  *mempool.IdentifierMap
	receiptIDsByResult *mempool.IdentifierMap
	headerStorage      *storage.Headers

	// resources fixtures
	collection      *flow.Collection
	block           *flow.Block
	receipt         *flow.ExecutionReceipt
	receiptDataPack *verification.ReceiptDataPack
	chunk           *flow.Chunk
	chunkDataPack   *flow.ChunkDataPack

	// identities
	verIdentity  *flow.Identity // verification node
	execIdentity *flow.Identity // execution node

	processInterval time.Duration

	// assertTimeOut is the timeout defined for asserting a call in test suite
	assertTimeOut time.Duration

	// other engine
	// mock Match engine, should be called when Finder engine completely
	// processes a receipt
	matchEng *network.Engine
}

// TestFinderEngine executes all FinderEngineTestSuite tests.
func TestFinderEngine(t *testing.T) {
	suite.Run(t, new(FinderEngineTestSuite))
}

// SetupTest initiates the test setups prior to each test.
func (suite *FinderEngineTestSuite) SetupTest() {
	suite.receiptsConduit = &network.Conduit{}
	suite.net = &module.Network{}
	suite.me = &module.Local{}
	suite.metrics = &module.VerificationMetrics{}
	suite.tracer = trace.NewNoopTracer()
	suite.headerStorage = &storage.Headers{}
	suite.cachedReceipts = &mempool.ReceiptDataPacks{}
	suite.pendingReceipts = &mempool.ReceiptDataPacks{}
	suite.readyReceipts = &mempool.ReceiptDataPacks{}
	suite.processedResultIDs = &mempool.Identifiers{}
	suite.blockIDsCache = &mempool.Identifiers{}
	suite.receiptIDsByBlock = &mempool.IdentifierMap{}
	suite.receiptIDsByResult = &mempool.IdentifierMap{}
	suite.matchEng = &network.Engine{}

	// generates an execution result with a single collection, chunk, and transaction.
	completeER := utils.LightExecutionResultFixture(1)
	suite.collection = completeER.Collections[0]
	suite.block = completeER.Block
	suite.receipt = completeER.Receipt
	suite.chunk = completeER.Receipt.ExecutionResult.Chunks[0]
	suite.chunkDataPack = completeER.ChunkDataPacks[0]

	suite.verIdentity = unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	suite.execIdentity = unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))

	suite.receiptDataPack = &verification.ReceiptDataPack{
		OriginID: suite.execIdentity.NodeID,
		Receipt:  suite.receipt,
	}

	suite.processInterval = 1 * time.Second
	// allows 5 process interval cycle before timeouting a call
	suite.assertTimeOut = 5 * suite.processInterval

	// mocking the network registration of the engine
	suite.net.On("Register", uint8(engine.ExecutionReceiptProvider), testifymock.Anything).
		Return(suite.receiptsConduit, nil).
		Once()

	// mocks identity of the verification node
	suite.me.On("NodeID").Return(suite.verIdentity.NodeID)
}

// TestNewFinderEngine tests the establishment of the network registration upon
// creation of an instance of FinderEngine using the New method.
// It also returns an instance of new engine to be used in the later tests.
func (suite *FinderEngineTestSuite) TestNewFinderEngine() *finder.Engine {
	e, err := finder.New(zerolog.Logger{},
		suite.metrics,
		suite.tracer,
		suite.net,
		suite.me,
		suite.matchEng,
		suite.cachedReceipts,
		suite.pendingReceipts,
		suite.readyReceipts,
		suite.headerStorage,
		suite.processedResultIDs,
		suite.receiptIDsByBlock,
		suite.receiptIDsByResult,
		suite.blockIDsCache,
		suite.processInterval)
	require.Nil(suite.T(), err, "could not create finder engine")

	suite.net.AssertExpectations(suite.T())

	return e
}

// TestHandleReceipt_HappyPath evaluates that handling a receipt that is not in the
// ready cache ends up the receipt being added to the receipt catch.
func (suite *FinderEngineTestSuite) TestHandleReceipt_HappyPath() {
	e := suite.TestNewFinderEngine()

	// mocks metrics
	// receiving an execution receipt
	suite.metrics.On("OnExecutionReceiptReceived").
		Return().Once()

	// mocks receipt being added to the cached receipts
	suite.cachedReceipts.On("Add", testifymock.AnythingOfType("*verification.ReceiptDataPack")).
		Return(true).Once()

	// sends receipt to finder engine
	err := e.Process(suite.execIdentity.NodeID, suite.receipt)
	require.NoError(suite.T(), err)

	testifymock.AssertExpectationsForObjects(suite.T(),
		suite.metrics,
		suite.cachedReceipts)
}

// TestHandleReceipt_Cached evaluates that handling a receipt that is already in the cache
// ends up the receipt being dropped.
func (suite *FinderEngineTestSuite) TestHandleReceipt_Cached() {
	e := suite.TestNewFinderEngine()

	// mocks metrics
	// receiving an execution receipt
	suite.metrics.On("OnExecutionReceiptReceived").
		Return().Once()

	// mocks receipt being added to the cached receipts
	suite.cachedReceipts.On("Add", testifymock.AnythingOfType("*verification.ReceiptDataPack")).
		Return(false).Once()

	// sends receipt to finder engine
	err := e.Process(suite.execIdentity.NodeID, suite.receipt)
	require.NoError(suite.T(), err)

	testifymock.AssertExpectationsForObjects(suite.T(),
		suite.metrics,
		suite.cachedReceipts)
}

// TestCachedToPending evaluates that having a cached receipt with its
// block not available results it moved to the pending mempool.
func (suite *FinderEngineTestSuite) TestCachedToPending() {
	e := suite.TestNewFinderEngine()

	// mocks a cached receipt
	suite.cachedReceipts.On("All").
		Return([]*verification.ReceiptDataPack{suite.receiptDataPack})

	// mocks no new finalized block
	suite.blockIDsCache.On("All").
		Return(flow.IdentifierList{})

	// mocks a receipt in ready mempool
	suite.readyReceipts.On("All").
		Return([]*verification.ReceiptDataPack{})

	// mocks result has not yet processed
	suite.processedResultIDs.On("Has", suite.receipt.ExecutionResult.ID()).
		Return(false).Once()

	// mocks block associated with receipt is not available
	suite.headerStorage.On("ByBlockID", suite.block.ID()).
		Return(nil, fmt.Errorf("block does not exist")).Once()

	// mocks adding receipt id to mapping mempool based on its result
	suite.receiptIDsByResult.On("Append", suite.receipt.ExecutionResult.ID(), suite.receipt.ID()).
		Return(true, nil).Once()

	// mocks adding receipt pending for block ID
	suite.receiptIDsByBlock.On("Append", suite.receipt.ExecutionResult.BlockID, suite.receipt.ID()).
		Return(true, nil).Once()

	// mocks moving from cached to pending
	moveWG := sync.WaitGroup{}
	moveWG.Add(2)
	// removing from cached
	suite.cachedReceipts.On("Rem", suite.receiptDataPack.Receipt.ID()).
		Run(func(args testifymock.Arguments) {
			moveWG.Done()
		}).Return(true).Once()

	// adding to pending
	suite.pendingReceipts.On("Add", suite.receiptDataPack).
		Run(func(args testifymock.Arguments) {
			moveWG.Done()
		}).Return(true).Once()

	// starts the engine
	<-e.Ready()

	// waits a timeout for finder engine to process receipt
	unittest.AssertReturnsBefore(suite.T(), moveWG.Wait, suite.assertTimeOut)

	// stops the engine
	<-e.Done()

	testifymock.AssertExpectationsForObjects(suite.T(),
		suite.readyReceipts,
		suite.cachedReceipts,
		suite.blockIDsCache,
		suite.metrics,
		suite.receiptIDsByResult,
		suite.matchEng)
}

// TestPendingToReady evaluates that having a pending receipt with its
// block becomes available results it moved to the ready mempool.
func (suite *FinderEngineTestSuite) TestPendingToReady() {
	e := suite.TestNewFinderEngine()

	// mocks no cached receipt
	suite.cachedReceipts.On("All").
		Return([]*verification.ReceiptDataPack{}).Once()

	// mocks a new finalized block
	suite.blockIDsCache.On("All").
		Return(flow.IdentifierList{suite.block.ID()}).Once()
	suite.blockIDsCache.On("Rem", suite.block.ID()).
		Return(true).Once()

	// mocks a receipt pending for this block
	suite.receiptIDsByBlock.On("Get", suite.block.ID()).
		Return([]flow.Identifier{suite.receiptDataPack.ID()}, true).Once()

	suite.receiptIDsByBlock.On("Rem", suite.block.ID()).
		Return(true).Once()

	// mocks a receipt in ready mempool
	suite.readyReceipts.On("All").
		Return([]*verification.ReceiptDataPack{})

	// mocks retrieving pending receipt
	suite.pendingReceipts.On("Get", suite.receipt.ID()).
		Return(suite.receiptDataPack, true).Once()

	// mocks moving from pending to ready
	moveWG := sync.WaitGroup{}
	moveWG.Add(2)
	// removing from pending
	suite.pendingReceipts.On("Rem", suite.receiptDataPack.Receipt.ID()).
		Run(func(args testifymock.Arguments) {
			moveWG.Done()
		}).Return(true).Once()

	// adding to ready
	suite.readyReceipts.On("Add", suite.receiptDataPack).
		Run(func(args testifymock.Arguments) {
			moveWG.Done()
		}).Return(true).Once()

	// starts the engine
	<-e.Ready()

	// waits a timeout for finder engine to process receipt
	unittest.AssertReturnsBefore(suite.T(), moveWG.Wait, suite.assertTimeOut)

	// stops the engine
	<-e.Done()

	testifymock.AssertExpectationsForObjects(suite.T(),
		suite.cachedReceipts,
		suite.blockIDsCache,
		suite.receiptIDsByBlock,
		suite.readyReceipts,
		suite.pendingReceipts)
}

// TestProcessReady_HappyPath evaluates that having a receipt in the ready mempool
// with its block available results in:
// - sending its result to match engine.
// - marking its result as processed.
// - removing it from mempool.
func (suite *FinderEngineTestSuite) TestProcessReady_HappyPath() {
	e := suite.TestNewFinderEngine()

	// mocks no receipt in cache
	suite.cachedReceipts.On("All").
		Return([]*verification.ReceiptDataPack{})

	// mocks no new finalized block
	suite.blockIDsCache.On("All").
		Return(flow.IdentifierList{})

	// mocks a receipt in ready mempool
	suite.readyReceipts.On("All").
		Return([]*verification.ReceiptDataPack{suite.receiptDataPack})

	// mocks result has not yet processed
	suite.processedResultIDs.On("Has", suite.receipt.ExecutionResult.ID()).
		Return(false).Once()

	// mocks successful submission to match engine
	matchWG := sync.WaitGroup{}
	matchWG.Add(1)
	suite.matchEng.On("Process", suite.execIdentity.NodeID, &suite.receipt.ExecutionResult).
		Run(func(args testifymock.Arguments) {
			matchWG.Done()
		}).Return(nil).Once()

	// mocks metrics
	// submitting a new execution result to match engine
	suite.metrics.On("OnExecutionResultSent").
		Return().Once()

	// mocks marking receipt as processed
	suite.processedResultIDs.On("Add", suite.receipt.ExecutionResult.ID()).
		Return(true).Once()

	// mocks receipt clean up after result is processed
	suite.receiptIDsByResult.On("Get", suite.receipt.ExecutionResult.ID()).
		Return([]flow.Identifier{suite.receipt.ID()}, true).Once()
	suite.receiptIDsByResult.On("Rem", suite.receipt.ExecutionResult.ID()).
		Return(true).Once()
	suite.readyReceipts.On("Rem", suite.receipt.ID()).
		Return(true).Once()

	// starts the engine
	<-e.Ready()

	// waits a timeout for finder engine to process receipt
	unittest.AssertReturnsBefore(suite.T(), matchWG.Wait, suite.assertTimeOut)

	// stops the engine
	<-e.Done()

	testifymock.AssertExpectationsForObjects(suite.T(),
		suite.readyReceipts,
		suite.cachedReceipts,
		suite.blockIDsCache,
		suite.processedResultIDs,
		suite.metrics,
		suite.receiptIDsByResult,
		suite.headerStorage,
		suite.matchEng)
}

// TestProcessReady_Retry evaluates failure in submission of an execution
// result to match engine results in retrying that receipt later on. In specific,
// the test evaluates retrying the receipt one more time.
func (suite *FinderEngineTestSuite) TestProcessReady_Retry() {
	e := suite.TestNewFinderEngine()
	retries := 2

	// mocks no receipt in cache
	suite.cachedReceipts.On("All").
		Return([]*verification.ReceiptDataPack{})

	// mocks no new finalized block
	suite.blockIDsCache.On("All").
		Return(flow.IdentifierList{})

	suite.readyReceipts.On("All").
		Return([]*verification.ReceiptDataPack{suite.receiptDataPack})

	// mocks result has not yet processed
	suite.processedResultIDs.On("Has", suite.receipt.ExecutionResult.ID()).
		Return(false).Times(retries)

	// mocks successful submission to match engine
	matchWG := sync.WaitGroup{}
	matchWG.Add(retries)
	suite.matchEng.On("Process", suite.execIdentity.NodeID, &suite.receipt.ExecutionResult).
		Run(func(args testifymock.Arguments) {
			matchWG.Done()
		}).Return(fmt.Errorf("submission error")).Times(retries)

	// these should not happen:
	// ready receipt with failure on submission should not be marked as processed
	suite.processedResultIDs.AssertNotCalled(suite.T(), "Add", suite.receipt.ExecutionResult.ID())
	// should not be any attempt to clean up resources
	suite.receiptIDsByResult.AssertNotCalled(suite.T(), "Get", suite.receipt.ExecutionResult.ID())
	suite.readyReceipts.AssertNotCalled(suite.T(), "Rem", suite.receipt.ID())
	// no metrics should be collected indicating a successful execution result submission
	suite.metrics.AssertNotCalled(suite.T(), "OnExecutionResultSent")

	// starts the engine
	<-e.Ready()

	// waits a timeout for finder engine to process receipt
	unittest.AssertReturnsBefore(suite.T(), matchWG.Wait, suite.assertTimeOut)

	// stops the engine
	<-e.Done()

	testifymock.AssertExpectationsForObjects(suite.T(),
		suite.readyReceipts,
		suite.cachedReceipts,
		suite.blockIDsCache,
		suite.processedResultIDs,
		suite.metrics,
		suite.receiptIDsByResult,
		suite.matchEng)
}

// TestHandleReceipt_DuplicateReady evaluates that trying to move a duplicate receipt from cached to
// ready status is dropped without attempting to process it.
func (suite *FinderEngineTestSuite) TestHandleReceipt_DuplicateReady() {
	e := suite.TestNewFinderEngine()

	// mocks a receipt in cache
	suite.cachedReceipts.On("All").
		Return([]*verification.ReceiptDataPack{suite.receiptDataPack})
	suite.cachedReceipts.On("Rem", suite.receiptDataPack.ID()).
		Return(true)

	// mocks no new finalized block
	suite.blockIDsCache.On("All").
		Return(flow.IdentifierList{})

	// mocks no new receipt
	suite.readyReceipts.On("All").
		Return([]*verification.ReceiptDataPack{})

	// mocks result has not yet processed
	suite.processedResultIDs.On("Has", suite.receipt.ExecutionResult.ID()).
		Return(false).Once()

	// mocks block associated with receipt is available
	suite.headerStorage.On("ByBlockID", suite.block.ID()).
		Return(&flow.Header{}, nil).Once()

	// mocks adding receipt to the ready receipts mempool returns a false result
	// (i.e., a duplicate exists)
	moveWG := sync.WaitGroup{}
	moveWG.Add(1)
	suite.readyReceipts.On("Add", suite.receiptDataPack).
		Return(false).Run(func(args testifymock.Arguments) {
		moveWG.Done()
	}).Once()

	// starts engine
	<-e.Ready()

	unittest.AssertReturnsBefore(suite.T(), moveWG.Wait, 5*time.Second)

	// terminates engine
	<-e.Done()

	// should not be any attempt on sending result to match engine
	suite.matchEng.AssertNotCalled(suite.T(), "Process", testifymock.Anything, testifymock.Anything)

	testifymock.AssertExpectationsForObjects(suite.T(),
		suite.cachedReceipts,
		suite.blockIDsCache,
		suite.readyReceipts,
		suite.processedResultIDs,
		suite.matchEng,
		suite.headerStorage)
}

// TestHandleReceipt_DuplicatePending evaluates that trying to move a duplicate receipt from cached to
// pending status is dropped without attempting to process it.
func (suite *FinderEngineTestSuite) TestHandleReceipt_DuplicatePending() {
	e := suite.TestNewFinderEngine()

	// mocks a receipt in cache
	suite.cachedReceipts.On("All").
		Return([]*verification.ReceiptDataPack{suite.receiptDataPack})
	suite.cachedReceipts.On("Rem", suite.receiptDataPack.ID()).
		Return(true)

	// mocks no new finalized block
	suite.blockIDsCache.On("All").
		Return(flow.IdentifierList{})

	// mocks no new ready receipt
	suite.readyReceipts.On("All").
		Return([]*verification.ReceiptDataPack{})

	// mocks result has not yet processed
	suite.processedResultIDs.On("Has", suite.receipt.ExecutionResult.ID()).
		Return(false).Once()

	// mocks block associated with receipt is not available
	suite.headerStorage.On("ByBlockID", suite.block.ID()).
		Return(nil, fmt.Errorf("no block")).Once()

	// mocks adding receipt to the pending receipts mempool returns a false result
	// (i.e., a duplicate exists)
	moveWG := sync.WaitGroup{}
	moveWG.Add(1)
	suite.pendingReceipts.On("Add", suite.receiptDataPack).
		Return(false).Run(func(args testifymock.Arguments) {
		moveWG.Done()
	}).Once()

	// starts engine
	<-e.Ready()

	unittest.AssertReturnsBefore(suite.T(), moveWG.Wait, 5*time.Second)

	// terminates engine
	<-e.Done()

	// should not be any attempt on sending result to match engine
	suite.matchEng.AssertNotCalled(suite.T(), "Process", testifymock.Anything, testifymock.Anything)

	testifymock.AssertExpectationsForObjects(suite.T(),
		suite.cachedReceipts,
		suite.blockIDsCache,
		suite.pendingReceipts,
		suite.readyReceipts,
		suite.processedResultIDs,
		suite.matchEng,
		suite.headerStorage)
}

// TestHandleReceipt_Processed evaluates that checking a cached receipt with a processed result
// is dropped without attempting to add it to any of ready and pending mempools
func (suite *FinderEngineTestSuite) TestHandleReceipt_Processed() {
	e := suite.TestNewFinderEngine()

	// mocks no new finalized block
	suite.blockIDsCache.On("All").
		Return(flow.IdentifierList{})

	// mocks no new ready receipt
	suite.readyReceipts.On("All").
		Return([]*verification.ReceiptDataPack{})

	// mocks a receipt in cache
	suite.cachedReceipts.On("All").
		Return([]*verification.ReceiptDataPack{suite.receiptDataPack})
	suite.cachedReceipts.On("Rem", suite.receiptDataPack.ID()).
		Return(true)

	// mocks result has already been processed
	moveWG := sync.WaitGroup{}
	moveWG.Add(1)
	suite.processedResultIDs.On("Has", suite.receipt.ExecutionResult.ID()).
		Run(func(args testifymock.Arguments) {
			moveWG.Done()
		}).Return(true).Once()

	// starts engine
	<-e.Ready()

	unittest.AssertReturnsBefore(suite.T(), moveWG.Wait, 5*time.Second)

	// terminates engine
	<-e.Done()

	// should not be any attempt on adding receipt to any of mempools
	suite.readyReceipts.AssertNotCalled(suite.T(), "Add", testifymock.Anything)
	suite.pendingReceipts.AssertNotCalled(suite.T(), "Add", testifymock.Anything)

	// should not be any attempt on sending result to match engine
	suite.matchEng.AssertNotCalled(suite.T(), "Process", testifymock.Anything, testifymock.Anything)

	testifymock.AssertExpectationsForObjects(suite.T(),
		suite.cachedReceipts,
		suite.blockIDsCache,
		suite.pendingReceipts,
		suite.readyReceipts,
		suite.processedResultIDs,
		suite.matchEng,
		suite.headerStorage)
}
