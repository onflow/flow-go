package ingest_test

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/verification"
	"github.com/dapperlabs/flow-go/engine/verification/ingest"
	"github.com/dapperlabs/flow-go/engine/verification/test"
	chmodel "github.com/dapperlabs/flow-go/model/chunks"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	verificationmodel "github.com/dapperlabs/flow-go/model/verification"
	"github.com/dapperlabs/flow-go/model/verification/tracker"
	mempool "github.com/dapperlabs/flow-go/module/mempool/mock"
	module "github.com/dapperlabs/flow-go/module/mock"
	network "github.com/dapperlabs/flow-go/network/mock"
	protocol "github.com/dapperlabs/flow-go/state/protocol/mock"
	storage "github.com/dapperlabs/flow-go/storage/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// TestSuite contains the context of a verifier engine test using mocked components.
type TestSuite struct {
	sync.Mutex // to provide mutual exclusion of mocked objects
	suite.Suite
	net   *module.Network
	state *protocol.State
	ss    *protocol.Snapshot
	me    *module.Local
	// mock conduit for requesting/receiving collections
	collectionsConduit *network.Conduit
	// mock conduit for requesting/receiving chunk states
	statesConduit *network.Conduit
	// mock conduit for receiving receipts
	receiptsConduit *network.Conduit
	// mock conduit for requesting/receiving chunk data packs
	chunksConduit *network.Conduit
	// mock verifier engine, should be called when all dependent resources
	// for a receipt have been received by the ingest engine.
	verifierEng *network.Engine
	// mock mempools used by the ingest engine, valid resources should be added
	// to these when they are received from an appropriate node role.
	authReceipts          *mempool.Receipts
	pendingReceipts       *mempool.PendingReceipts
	authCollections       *mempool.Collections
	pendingCollections    *mempool.PendingCollections
	collectionTrackers    *mempool.CollectionTrackers
	chunkDataPacks        *mempool.ChunkDataPacks
	chunkDataPackTrackers *mempool.ChunkDataPackTrackers
	ingestedChunkIDs      *mempool.Identifiers
	ingestedResultIDs     *mempool.Identifiers
	blockStorage          *storage.Blocks
	// resources fixtures
	collection       *flow.Collection
	block            *flow.Block
	receipt          *flow.ExecutionReceipt
	chunk            *flow.Chunk
	chunkDataPack    *flow.ChunkDataPack
	chunkTracker     *tracker.ChunkDataPackTracker
	assigner         *module.ChunkAssigner
	collTracker      *tracker.CollectionTracker
	requestInterval  uint
	failureThreshold uint
}

// TestIngestEngine executes all TestSuite tests.
func TestIngestEngine(t *testing.T) {
	suite.Run(t, new(TestSuite))
}

// SetupTest initiates the test setups prior to each test.
func (suite *TestSuite) SetupTest() {
	// initializing test suite fields
	suite.collectionsConduit = &network.Conduit{}
	suite.statesConduit = &network.Conduit{}
	suite.receiptsConduit = &network.Conduit{}
	suite.chunksConduit = &network.Conduit{}
	suite.net = &module.Network{}
	suite.verifierEng = &network.Engine{}

	suite.state = &protocol.State{}
	suite.me = &module.Local{}
	suite.ss = &protocol.Snapshot{}
	suite.blockStorage = &storage.Blocks{}
	suite.authReceipts = &mempool.Receipts{}
	suite.pendingReceipts = &mempool.PendingReceipts{}
	suite.authCollections = &mempool.Collections{}
	suite.pendingCollections = &mempool.PendingCollections{}
	suite.collectionTrackers = &mempool.CollectionTrackers{}
	suite.chunkDataPacks = &mempool.ChunkDataPacks{}
	suite.chunkDataPackTrackers = &mempool.ChunkDataPackTrackers{}
	suite.ingestedResultIDs = &mempool.Identifiers{}
	suite.ingestedChunkIDs = &mempool.Identifiers{}
	suite.assigner = &module.ChunkAssigner{}

	completeER := test.CompleteExecutionResultFixture(suite.T(), 1)
	suite.collection = completeER.Collections[0]
	suite.block = completeER.Block
	suite.receipt = completeER.Receipt
	suite.chunk = completeER.Receipt.ExecutionResult.Chunks[0]
	suite.chunkDataPack = completeER.ChunkDataPacks[0]
	suite.collTracker = tracker.NewCollectionTracker(suite.collection.ID(), suite.block.ID())
	suite.chunkTracker = tracker.NewChunkDataPackTracker(suite.chunk.ID(), suite.block.ID())

	// parameters set based on following issue
	// https://github.com/dapperlabs/flow-go/issues/3443
	suite.failureThreshold = 2
	suite.requestInterval = 1000

	// mocking the network registration of the engine
	// all subsequent tests are expected to have a call on Register method
	suite.net.On("Register", uint8(engine.CollectionProvider), testifymock.Anything).
		Return(suite.collectionsConduit, nil).
		Once()
	suite.net.On("Register", uint8(engine.ExecutionReceiptProvider), testifymock.Anything).
		Return(suite.receiptsConduit, nil).
		Once()
	suite.net.On("Register", uint8(engine.ExecutionStateProvider), testifymock.Anything).
		Return(suite.statesConduit, nil).
		Once()
	suite.net.On("Register", uint8(engine.ChunkDataPackProvider), testifymock.Anything).
		Return(suite.chunksConduit, nil).
		Once()
}

// TestNewEngine verifies the establishment of the network registration upon
// creation of an instance of verifier.IngestEngine using the New method
// It also returns an instance of new engine to be used in the later tests
func (suite *TestSuite) TestNewEngine() *ingest.Engine {
	e, err := ingest.New(zerolog.Logger{},
		suite.net,
		suite.state,
		suite.me,
		suite.verifierEng,
		suite.authReceipts,
		suite.pendingReceipts,
		suite.authCollections,
		suite.pendingCollections,
		suite.collectionTrackers,
		suite.chunkDataPacks,
		suite.chunkDataPackTrackers,
		suite.ingestedChunkIDs,
		suite.ingestedResultIDs,
		suite.blockStorage,
		suite.assigner,
		suite.requestInterval,
		suite.failureThreshold)
	require.Nil(suite.T(), err, "could not create an engine")

	suite.net.AssertExpectations(suite.T())

	return e
}

// TestHandleBlock passes a block to ingest engine and evaluates internal path
// as ingest engine only accepts a block through consensus follower, it should return an error
func (suite *TestSuite) TestHandleBlock() {
	// locks to run the test sequentially
	suite.Lock()
	defer suite.Unlock()

	eng := suite.TestNewEngine()
	err := eng.Process(unittest.IdentifierFixture(), suite.block)
	assert.Equal(suite.T(), err, ingest.ErrInvType)
}

// TestHandleReceipt_MissingCollection evaluates that when ingest engine has both a receipt and its block
// but not the collections, it asks for the collections through the network
func (suite *TestSuite) TestHandleReceipt_MissingCollection() {
	// locks to run the test sequentially
	suite.Lock()
	defer suite.Unlock()

	eng := suite.TestNewEngine()

	// mocks identities
	//
	// required roles
	execIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	verIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	collIdentities := unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleCollection))

	// mocks identity of the verification node
	suite.me.On("NodeID").Return(verIdentity.NodeID)

	// mocks state snapshot to validate identity of execution node as an staked origin id at the `suite.block` height
	suite.state.On("Final").Return(suite.ss, nil)
	suite.state.On("AtBlockID", suite.block.ID()).Return(suite.ss, nil).Once()
	suite.ss.On("Identity", execIdentity.NodeID).Return(execIdentity, nil).Once()
	// mocks state snapshot to return collIdentities as identity list of staked collection nodes
	suite.ss.On("Identities", testifymock.AnythingOfType("flow.IdentityFilter")).Return(collIdentities, nil).Twice()

	// mocks existing resources at the engine's disposal
	//
	// mocks the possession of 'suite.block` in the storage
	suite.blockStorage.On("ByID", suite.block.ID()).Return(suite.block, nil).Once()
	// mocks the possession of chunk data pack associated with the `suite.block`
	suite.chunkDataPacks.On("Has", suite.chunkDataPack.ID()).Return(true).Once()
	suite.chunkDataPacks.On("ByChunkID", suite.chunkDataPack.ID()).Return(suite.chunkDataPack, nil).Once()

	// mocks missing collection
	//
	// mocks the absence of `suite.collection` which is the associated collection to this block
	// the collection does not exist in authenticated and pending collections mempools
	suite.authCollections.On("Has", suite.collection.ID()).Return(false).Once()
	suite.pendingCollections.On("Has", suite.collection.ID()).Return(false).Once()

	// engine has not yet ingested the result of this receipt yet
	suite.ingestedResultIDs.On("Has", suite.receipt.ExecutionResult.ID()).Return(false)

	for _, chunk := range suite.receipt.ExecutionResult.Chunks {
		suite.ingestedChunkIDs.On("Has", chunk.ID()).Return(false)
	}

	// expect that we already have the receipt in the authenticated receipts mempool
	suite.authReceipts.On("All").Return([]*flow.ExecutionReceipt{suite.receipt}, nil).Once()
	suite.pendingReceipts.On("All").Return([]*verificationmodel.PendingReceipt{}).Once()
	suite.authReceipts.On("Add", suite.receipt).Return(nil).Once()

	// mocks functionalities
	//
	// adding functionality of chunk tracker to trackers mempool
	// mocks initial insertion of tracker into mempool
	suite.collectionTrackers.On("Add", suite.collTracker).Return(nil).Once()
	// there is no tracker registered for the collection, i.e., the collection has not been requested yet
	suite.collectionTrackers.On("Has", suite.collection.ID()).Return(false)

	suite.collectionsConduit.
		On("Submit", testifymock.AnythingOfType("*messages.CollectionRequest"), collIdentities[0].NodeID).
		Return(nil).Once()

	// mocks chunk assignment
	//
	// assigns all chunks in the receipt to this node through mocking
	a := chmodel.NewAssignment()
	for _, chunk := range suite.receipt.ExecutionResult.Chunks {
		a.Add(chunk, []flow.Identifier{verIdentity.NodeID})
	}
	suite.assigner.On("Assign",
		testifymock.Anything,
		testifymock.Anything,
		testifymock.Anything).
		Return(a, nil).
		Once()

	err := eng.Process(execIdentity.NodeID, suite.receipt)
	suite.Assert().Nil(err)

	// asserts necessary calls
	suite.authReceipts.AssertExpectations(suite.T())
	suite.collectionsConduit.AssertExpectations(suite.T())
	suite.assigner.AssertExpectations(suite.T())
	suite.ss.AssertExpectations(suite.T())
	suite.state.AssertExpectations(suite.T())

	// verifier should not be called
	suite.verifierEng.AssertNotCalled(suite.T(), "ProcessLocal", testifymock.Anything)
}

// TestHandleReceipt_MissingChunkDataPack evaluates that when ingest engine has both a receipt and its block
// but not the chunk data pack of it, it asks for the chunk data pack through the network
func (suite *TestSuite) TestHandleReceipt_MissingChunkDataPack() {
	// locks to run the test sequentially
	suite.Lock()
	defer suite.Unlock()

	eng := suite.TestNewEngine()

	// mocks identities
	//
	// required roles
	execIdentities := unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleExecution))
	verIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	// mocks identity of the verification node
	suite.me.On("NodeID").Return(verIdentity.NodeID)

	// mocks state snapshot
	//
	// mocks state snapshot to validate identity of execution node as an staked origin id at the `suite.block` height
	suite.state.On("Final").Return(suite.ss, nil)
	suite.state.On("AtBlockID", suite.block.ID()).Return(suite.ss, nil).Once()
	suite.ss.On("Identity", execIdentities[0].NodeID).Return(execIdentities[0], nil).Once()
	// mocks state snapshot to return exeIdentities as identity list of staked collection nodes
	suite.ss.On("Identities", testifymock.AnythingOfType("flow.IdentityFilter")).Return(execIdentities, nil).Twice()

	// mocks existing resources at the engine's disposal
	//
	// block
	suite.blockStorage.On("ByID", suite.block.ID()).Return(suite.block, nil).Once()
	// collection
	suite.authCollections.On("Has", suite.collection.ID()).Return(true).Once()
	// receipt in the authenticated mempool
	suite.authReceipts.On("All").Return([]*flow.ExecutionReceipt{suite.receipt}, nil).Once()
	suite.pendingReceipts.On("All").Return([]*verificationmodel.PendingReceipt{}).Once()

	// mocks missing resources
	//
	// absence of chunk data pack itself
	suite.chunkDataPacks.On("Has", suite.chunkDataPack.ID()).Return(false).Once()
	// engine has not yet ingested the result of this receipt as well as its chunks yet
	suite.ingestedResultIDs.On("Has", suite.receipt.ExecutionResult.ID()).Return(false)
	suite.ingestedChunkIDs.On("Has", suite.chunk.ID()).Return(false)

	// mocks chunk assignment
	//
	// assigns all chunks in the receipt to this node through mocking
	a := chmodel.NewAssignment()
	for _, chunk := range suite.receipt.ExecutionResult.Chunks {
		a.Add(chunk, []flow.Identifier{verIdentity.NodeID})
	}
	suite.assigner.On("Assign",
		testifymock.Anything,
		testifymock.Anything,
		testifymock.Anything).
		Return(a, nil).
		Once()

	// mocks functionalities
	//
	// adding functionality of chunk tracker to trackers mempool
	// mocks initial insertion of tracker into mempool
	suite.chunkDataPackTrackers.On("Add", suite.chunkTracker).Return(nil).Once()
	// mocks tracker check
	// absence of a tracker for chunk data pack
	suite.chunkDataPackTrackers.On("Has", suite.chunkDataPack.ID()).Return(false)
	// mocks the functionality of adding receipt to the mempool
	suite.authReceipts.On("Add", suite.receipt).Return(nil).Once()

	suite.chunksConduit.
		On("Submit", testifymock.AnythingOfType("*messages.ChunkDataPackRequest"), execIdentities[0].NodeID).Return(nil).Once()

	err := eng.Process(execIdentities[0].NodeID, suite.receipt)
	suite.Assert().Nil(err)

	// asserts necessary calls
	suite.chunksConduit.AssertExpectations(suite.T())
	suite.chunkDataPackTrackers.AssertExpectations(suite.T())
	suite.assigner.AssertExpectations(suite.T())
	suite.ss.AssertExpectations(suite.T())
	suite.state.AssertExpectations(suite.T())

	// verifier should not be called
	suite.verifierEng.AssertNotCalled(suite.T(), "ProcessLocal", testifymock.Anything)
}

// TestHandleReceipt_RetryMissingCollection evaluates that when ingest engine has a missing collections with
// a tracker registered, it retries its request (`failureThreshold` - 1)-many times and then drops it.
// The -1 is to account for the initial request of the collection directly without registering the tracker.
func (suite *TestSuite) TestHandleReceipt_RetryMissingCollection() {
	// locks to run the test sequentially
	suite.Lock()
	defer suite.Unlock()

	eng := suite.TestNewEngine()

	// mocks identities
	//
	// required roles
	collIdentities := unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleCollection))

	// mocking state
	//
	// mocks state snapshot to return collIdentities as identity list of staked collection nodes
	suite.state.On("Final").Return(suite.ss, nil)
	suite.ss.On("Identities", testifymock.AnythingOfType("flow.IdentityFilter")).Return(collIdentities, nil)

	// mocks functionalities
	//
	// mocks tracker check
	// presence of tracker in the trackers mempool
	suite.collectionTrackers.On("Has", suite.collection.ID()).Return(true)
	suite.collectionTrackers.On("All").Return([]*tracker.CollectionTracker{suite.collTracker})
	// update functionality for the present tracker
	suite.collectionTrackers.On("Add", suite.collTracker).Run(func(args testifymock.Arguments) {
		// +1 accounts for updating the trackers counter
		suite.collTracker.Counter += 1
	}).Return(nil)
	suite.collectionTrackers.On("ByCollectionID", suite.collTracker.ID()).Return(suite.collTracker, nil)
	suite.collectionTrackers.On("Rem", suite.collTracker.ID()).Return(true)

	// no chunl data pack tracjer
	suite.chunkDataPackTrackers.On("All").Return(nil)

	// mocks expectation
	//
	// expect that the collection is requested from collection nodes `failureThreshold` - 1 many times
	// the -1 is to exclude the initial request submission made before adding tracker to mempool
	submitWG := sync.WaitGroup{}
	submitWG.Add(int(suite.failureThreshold) - 1)
	suite.collectionsConduit.
		On("Submit", testifymock.AnythingOfType("*messages.CollectionRequest"), collIdentities[0].NodeID).
		Run(func(args testifymock.Arguments) {
			submitWG.Done()
		}).
		Return(nil)

	// starts engine
	<-eng.Ready()

	// starts timer for submitting retries
	// expects `failureThreshold`-many requests each sent at `requestInterval` milliseconds time interval
	unittest.RequireReturnsBefore(suite.T(), submitWG.Wait,
		time.Duration(int64(suite.failureThreshold*suite.requestInterval))*time.Millisecond)

	// waits for the engine to get shutdown
	<-eng.Done()

	// verifier should not be called
	suite.verifierEng.AssertNotCalled(suite.T(), "ProcessLocal", testifymock.Anything)
}

// TestHandleReceipt_RetryMissingChunkDataPack evaluates that when ingest engine has a missing chunk data pack with
// a tracker registered, it retries its request (`failureThreshold` - 1)-many times and then drops it.
// The -1 is to account for the initial request of the chunk data pack directly without registering the tracker.
func (suite *TestSuite) TestHandleReceipt_RetryMissingChunkDataPack() {
	// locks to run the test sequentially
	suite.Lock()
	defer suite.Unlock()

	eng := suite.TestNewEngine()

	// mocks identities
	//
	// required roles
	execIdentities := unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleExecution))

	// mocks state
	//
	// mocks state snapshot to return exeIdentities as identity list of staked execution nodes
	suite.state.On("Final").Return(suite.ss, nil)
	suite.ss.On("Identities", testifymock.AnythingOfType("flow.IdentityFilter")).Return(execIdentities, nil)

	// mocks functionalities
	//
	// mocks tracker check
	// presence of tracker in the trackers mempool
	suite.chunkDataPackTrackers.On("Has", suite.chunkDataPack.ID()).Return(true)
	suite.chunkDataPackTrackers.On("All").Return([]*tracker.ChunkDataPackTracker{suite.chunkTracker})
	// update functionality for the present tracker
	suite.chunkDataPackTrackers.On("Add", suite.chunkTracker).Run(func(args testifymock.Arguments) {
		// +1 accounts for updating the trackers counter
		suite.chunkTracker.Counter += 1
	}).Return(nil)
	suite.chunkDataPackTrackers.On("ByChunkID", suite.chunkTracker.ChunkID).Return(suite.chunkTracker, nil).Once()
	suite.chunkDataPackTrackers.On("Rem", suite.chunkTracker.ChunkID).Return(true).Once()

	// no collection tracker
	suite.collectionTrackers.On("All").Return(nil)

	// mocks expectation
	//
	// expect that the chunk data pack is requested from execution nodes `failureThreshold` - 1 many times
	// the -1 is to exclude the initial request submission made before adding tracker to mempool
	submitWG := sync.WaitGroup{}
	submitWG.Add(int(suite.failureThreshold) - 1)
	suite.chunksConduit.
		On("Submit", testifymock.AnythingOfType("*messages.ChunkDataPackRequest"), execIdentities[0].NodeID).
		Run(func(args testifymock.Arguments) {
			submitWG.Done()
		}).Return(nil).Once()

	// starts engine
	<-eng.Ready()

	// starts timer for submitting retries
	// expects `failureThreshold`-many requests each sent at `requestInterval` milliseconds time interval
	unittest.RequireReturnsBefore(suite.T(), submitWG.Wait,
		time.Duration(int64(suite.failureThreshold*suite.requestInterval))*time.Millisecond)

	// waits for the engine to get shutdown
	<-eng.Done()

	// verifier should not be called
	suite.verifierEng.AssertNotCalled(suite.T(), "ProcessLocal", testifymock.Anything)
}

// TestIngestedResult evaluates the happy path of submitting an execution receipt with an already ingested result
func (suite *TestSuite) TestIngestedResult() {
	// locks to run the test sequentially
	suite.Lock()
	defer suite.Unlock()

	eng := suite.TestNewEngine()

	// mocks this receipt's result as ingested
	suite.ingestedResultIDs.On("Has", suite.receipt.ExecutionResult.ID()).Return(true)

	// nothing else is mocked, hence the process should simply return nil
	err := eng.Process(unittest.IdentifierFixture(), suite.receipt)
	require.NoError(suite.T(), err)
}

// TestIngestedChunk evaluates the happy path of submitting a chunk data pack for an already ingested chunk
func (suite *TestSuite) TestIngestedChunk() {
	// locks to run the test sequentially
	suite.Lock()
	defer suite.Unlock()

	eng := suite.TestNewEngine()

	// creates a chunk fixture, its data pack, and the data pack response
	chunk := unittest.ChunkFixture()
	chunkDataPack := unittest.ChunkDataPackFixture(chunk.ID())
	chunkDataPackResponse := &messages.ChunkDataPackResponse{Data: chunkDataPack}
	// mocks this chunk id
	suite.ingestedChunkIDs.On("Has", chunkDataPack.ChunkID).Return(true)

	// nothing else is mocked, hence the process should simply return nil
	err := eng.Process(unittest.IdentifierFixture(), chunkDataPackResponse)
	require.NoError(suite.T(), err)
}

// TestHandleReceipt_UnstakedSender evaluates sending an execution receipt from an unstaked node
// it should go to the pending receipts and (later on) dropped from the cache
// Todo dropping unauthenticated receipts from cache
// https://github.com/dapperlabs/flow-go/issues/2966
func (suite *TestSuite) TestHandleReceipt_UnstakedSender() {
	// locks to run the test sequentially
	suite.Lock()
	defer suite.Unlock()

	eng := suite.TestNewEngine()

	myIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	suite.me.On("NodeID").Return(myIdentity.NodeID).Once()

	// mock the receipt coming from an unstaked node
	unstakedIdentity := unittest.IdentifierFixture()
	suite.state.On("Final").Return(suite.ss).Once()
	suite.state.On("AtBlockID", testifymock.Anything).Return(suite.ss, nil).Once()
	suite.ss.On("Identity", unstakedIdentity).Return(nil, fmt.Errorf("unstaked node")).Once()
	// engine has not yet ingested the result of this receipt
	suite.ingestedResultIDs.On("Has", suite.receipt.ExecutionResult.ID()).Return(false)

	// receipt should go to the pending receipts mempool
	suite.pendingReceipts.On("Add", suite.receipt).Return(nil).Once()

	// creates and mocks a pending receipt for the unstaked node
	p := &verificationmodel.PendingReceipt{
		Receipt:  suite.receipt,
		OriginID: unstakedIdentity,
	}
	preceipts := []*verificationmodel.PendingReceipt{p}

	// receipt should go to the pending receipts mempool
	suite.pendingReceipts.On("Add", p).Return(nil).Once()
	suite.pendingReceipts.On("All").Return(preceipts).Once()
	suite.authReceipts.On("All").Return([]*flow.ExecutionReceipt{}).Once()
	suite.blockStorage.On("ByID", suite.block.ID()).Return(nil, fmt.Errorf("block does not exist")).Once()

	err := eng.Process(unstakedIdentity, suite.receipt)
	require.NoError(suite.T(), err)

	// receipt should not be added
	suite.authReceipts.AssertNotCalled(suite.T(), "Add", suite.receipt)

	// Todo (depends on handling edge cases): adding network call for requesting block of receipt from consensus nodes

	// verifier should not be called
	suite.verifierEng.AssertNotCalled(suite.T(), "ProcessLocal", testifymock.Anything)
}

func (suite *TestSuite) TestHandleReceipt_SenderWithWrongRole() {
	invalidRoles := []flow.Role{flow.RoleConsensus, flow.RoleCollection, flow.RoleVerification, flow.RoleAccess}

	for _, role := range invalidRoles {
		suite.Run(fmt.Sprintf("role: %s", role), func() {
			// locks to run the test cases sequentially
			suite.Lock()
			defer suite.Unlock()

			// refresh test state in between each loop
			suite.SetupTest()
			eng := suite.TestNewEngine()

			// mock the receipt coming from the invalid role
			invalidIdentity := unittest.IdentityFixture(unittest.WithRole(role))
			suite.state.On("Final").Return(suite.ss)
			suite.state.On("AtBlockID", testifymock.Anything).Return(suite.ss, nil)
			suite.ss.On("Identity", invalidIdentity.NodeID).Return(invalidIdentity, nil).Once()

			receipt := unittest.ExecutionReceiptFixture()

			// process should fail
			err := eng.Process(invalidIdentity.NodeID, &receipt)
			suite.Assert().Error(err)

			// receipt should not be added
			suite.authReceipts.AssertNotCalled(suite.T(), "Add", &receipt)

			// verifier should not be called
			suite.verifierEng.AssertNotCalled(suite.T(), "ProcessLocal", testifymock.Anything)
		})
	}

}

// TestHandleCollection_Tracked evaluates receiving a tracked collection without any other receipt-dependent resources
// the collection should be added to the authenticate collection pool, and tracker should be removed
func (suite *TestSuite) TestHandleCollection_Tracked() {
	// locks to run the tests sequentially
	suite.Lock()
	defer suite.Unlock()

	eng := suite.TestNewEngine()

	// mock the collection coming from an collection node
	collIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))

	suite.authReceipts.On("All").Return([]*flow.ExecutionReceipt{}, nil)
	suite.pendingReceipts.On("All").Return([]*verificationmodel.PendingReceipt{}, nil)
	suite.collectionTrackers.On("Has", suite.collection.ID()).Return(true)
	suite.collectionTrackers.On("ByCollectionID", suite.collection.ID()).Return(suite.collTracker, nil)
	suite.state.On("Final").Return(suite.ss).Once()
	suite.state.On("AtBlockID", testifymock.Anything).Return(suite.ss, nil)
	suite.ss.On("Identity", collIdentity.NodeID).Return(collIdentity, nil).Once()

	// expect that the collection be added to the mempool
	suite.authCollections.On("Add", suite.collection).Return(nil).Once()

	// expect that the collection tracker is removed
	suite.collectionTrackers.On("Rem", suite.collection.ID()).Return(true).Once()

	err := eng.Process(collIdentity.NodeID, suite.collection)
	suite.Assert().Nil(err)

	suite.authCollections.AssertExpectations(suite.T())
	suite.collectionTrackers.AssertExpectations(suite.T())

	// verifier should not be called
	suite.verifierEng.AssertNotCalled(suite.T(), "ProcessLocal", testifymock.Anything)
}

// TestHandleCollection_Untracked evaluates receiving an  un-tracked collection
// It expects that the collection to be added to the pending receipts
func (suite *TestSuite) TestHandleCollection_Untracked() {
	// Locks to run the tests sequentially
	suite.Lock()
	defer suite.Unlock()

	eng := suite.TestNewEngine()

	// mock the collection coming from an collection node
	collIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	suite.collectionTrackers.On("ByCollectionID", suite.collection.ID()).
		Return(nil, fmt.Errorf("does not exit")).Once()
	// mocks a pending collection
	pcoll := &verificationmodel.PendingCollection{
		Collection: suite.collection,
		OriginID:   collIdentity.NodeID,
	}
	// expects the the collection to be added to pending receipts
	suite.pendingCollections.On("Add", pcoll).Return(nil).Once()

	err := eng.Process(collIdentity.NodeID, suite.collection)
	suite.Assert().Nil(err)

	suite.authCollections.AssertExpectations(suite.T())
	suite.collectionTrackers.AssertExpectations(suite.T())

	// verifier should not be called
	suite.verifierEng.AssertNotCalled(suite.T(), "ProcessLocal", testifymock.Anything)
	// does not expect that the collection be added to the mempool
	suite.authCollections.AssertNotCalled(suite.T(), "Add", suite.collection)
	// does not expect tracker to be removed from trackers mempool
	suite.collectionTrackers.AssertNotCalled(suite.T(), "Rem", suite.collection.ID())
}

// TestHandleCollection_UnstakedSender evaluates receiving a tracked collection from an unstaked node
// process method should return an error
// TODO pending collections cleanup
// https://github.com/dapperlabs/flow-go/issues/2966
func (suite *TestSuite) TestHandleCollection_UnstakedSender() {
	// locks to run the tests sequentially
	suite.Lock()
	defer suite.Unlock()

	eng := suite.TestNewEngine()

	// mock the receipt coming from an unstaked node
	unstakedIdentity := unittest.IdentifierFixture()
	suite.state.On("Final").Return(suite.ss).Once()
	suite.state.On("AtBlockID", testifymock.Anything).Return(suite.ss, nil)
	suite.ss.On("Identity", unstakedIdentity).Return(nil, errors.New("")).Once()

	// mocks a tracker for the collection
	suite.collectionTrackers.On("ByCollectionID", suite.collection.ID()).Return(suite.collTracker, nil)

	err := eng.Process(unstakedIdentity, suite.collection)
	suite.Assert().Error(err)

	// should not add collection to mempool
	suite.authCollections.AssertNotCalled(suite.T(), "Add", suite.collection)

	// should not call verifier
	suite.verifierEng.AssertNotCalled(suite.T(), "ProcessLocal", testifymock.Anything)

}

// TestHandleCollection_UnstakedSender evaluates receiving a tracked collection from an unstaked node
// process method should return an error
func (suite *TestSuite) TestHandleCollection_SenderWithWrongRole() {
	invalidRoles := []flow.Role{flow.RoleConsensus, flow.RoleExecution, flow.RoleVerification, flow.RoleAccess}

	for _, role := range invalidRoles {
		// locks to run the test sequentially
		suite.Lock()

		// refresh test state in between each loop
		suite.SetupTest()
		eng := suite.TestNewEngine()

		// mock the collection coming from the invalid role
		invalidIdentity := unittest.IdentityFixture(unittest.WithRole(role))
		suite.state.On("Final").Return(suite.ss).Once()
		suite.state.On("AtBlockID", testifymock.Anything).Return(suite.ss, nil)
		suite.ss.On("Identity", invalidIdentity.NodeID).Return(invalidIdentity, nil).Once()
		// mocks a tracker for the collection
		suite.collectionTrackers.On("ByCollectionID", suite.collection.ID()).Return(suite.collTracker, nil)

		err := eng.Process(invalidIdentity.NodeID, suite.collection)
		suite.Assert().Error(err)

		// should not add collection to mempool
		suite.authCollections.AssertNotCalled(suite.T(), "Add", suite.collection)

		suite.Unlock()
	}
}

// TestVerifyReady evaluates that a verifiable chunk is locally passed to the verifier engine
// whenever all of its relevant resources are ready regardless of the order in which dependent resources are received.
func (suite *TestSuite) TestVerifyReady() {
	// Mocking identities
	//
	// required roles
	execIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	collIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	verIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

	testcases := []struct {
		getResource func(*TestSuite) interface{}
		from        *flow.Identity
		label       string
	}{
		{
			getResource: func(s *TestSuite) interface{} { return s.receipt },
			from:        execIdentity,
			label:       "received receipt",
		}, {
			getResource: func(s *TestSuite) interface{} { return s.collection },
			from:        collIdentity,
			label:       "received collection",
		},
	}

	for _, testcase := range testcases {
		suite.Run(testcase.label, func() {
			// locks to run the tests sequentially
			suite.Lock()
			defer suite.Unlock()

			suite.SetupTest()
			eng := suite.TestNewEngine()
			// mocks state snapshot to validate identity of test case origin
			// as an staked origin id at the `suite.block` height
			suite.state.On("Final").Return(suite.ss, nil)
			suite.state.On("AtBlockID", suite.block.ID()).Return(suite.ss, nil)
			// mocks state snapshot to return identity of this verifier node for chunk assignment
			suite.ss.On("Identities", testifymock.AnythingOfType("flow.IdentityFilter")).Return(flow.IdentityList{verIdentity}, nil)
			// mocks state snapshot to return id of this verifier node
			suite.me.On("NodeID").Return(verIdentity.NodeID)

			// mocks identity of the origin id of test case
			suite.ss.On("Identity", testcase.from.NodeID).Return(testcase.from, nil)

			// mocks the functionality of adding collection to the mempool
			suite.authCollections.On("Add", suite.collection).Return(nil)

			// mocks missing resources
			//
			// engine has not yet ingested this receipt and its chunk
			suite.ingestedResultIDs.On("Has", suite.receipt.ExecutionResult.ID()).
				Return(false).Once()
			for _, chunk := range suite.receipt.ExecutionResult.Chunks {
				suite.ingestedChunkIDs.On("Has", chunk.ID()).Return(false)
				suite.ingestedChunkIDs.On("Add", chunk.ID()).Return(nil)
			}

			// mocks available resources at engine's disposal
			//
			// block
			suite.blockStorage.On("ByID", suite.block.ID()).Return(suite.block, nil)
			// collection
			suite.authCollections.On("Has", suite.collection.ID()).Return(true)
			suite.authCollections.On("ByID", suite.collection.ID()).Return(suite.collection, nil)
			// tracker for the collection
			suite.collectionTrackers.On("Has", suite.collection.ID()).Return(true)
			suite.collectionTrackers.On("ByCollectionID", suite.collection.ID()).Return(suite.collTracker, nil)
			// chunk data pack in mempool
			suite.chunkDataPacks.On("Has", suite.chunkDataPack.ID()).Return(true)
			suite.chunkDataPacks.On("ByChunkID", suite.chunkDataPack.ID()).Return(suite.chunkDataPack, nil)
			// execution receipt in authenticated pool
			suite.authReceipts.On("Add", suite.receipt).Return(nil)
			suite.authReceipts.On("All").Return([]*flow.ExecutionReceipt{suite.receipt}, nil)
			// pending receipt for the test case in mempool
			p := &verificationmodel.PendingReceipt{
				Receipt:  suite.receipt,
				OriginID: testcase.from.NodeID,
			}
			preceipts := []*verificationmodel.PendingReceipt{p}
			suite.pendingReceipts.On("All").Return(preceipts)

			// mocks cleanup functionalities
			//
			// mocks removing collection from authenticated collections
			suite.authCollections.On("Rem", suite.collection.ID()).Return(true)
			// mocks removing chunk data pack from the mempool
			suite.chunkDataPacks.On("Rem", suite.chunkDataPack.ID()).Return(true)
			// mocks removing collection tracker from mempool
			suite.collectionTrackers.On("Rem", suite.collection.ID()).Return(true)
			// mocks removing receipt from pending mempool
			suite.pendingReceipts.On("Rem", suite.receipt.ID()).Return(true)

			// mocks chunk assignment
			//
			// assigns the only chunk of the receipt to this verification node
			a := chmodel.NewAssignment()
			chunk, ok := suite.receipt.ExecutionResult.Chunks.ByIndex(0)
			require.True(suite.T(), ok, "chunk out of range requested")
			a.Add(chunk, flow.IdentifierList{verIdentity.NodeID})
			suite.assigner.On("Assign",
				testifymock.Anything,
				testifymock.Anything,
				testifymock.Anything).Return(a, nil)

			// mocks test expectation
			//
			// verifier engine should get called locally by a verifiable chunk
			// also checks the end state of verifiable chunks about edge cases
			suite.verifierEng.On("ProcessLocal", testifymock.AnythingOfType("*verification.VerifiableChunk")).
				Run(func(args testifymock.Arguments) {
					// the received entity should be a verifiable chunk
					vc, ok := args[0].(*verification.VerifiableChunk)
					assert.True(suite.T(), ok)

					// checks verifiable chunk end state
					// it should be the same as final state of receipt
					// since this ER has only a single chunk
					// more chunks cases are covered in concurrency_test
					if !bytes.Equal(vc.EndState, suite.receipt.ExecutionResult.FinalStateCommit) {
						assert.Fail(suite.T(), "last chunk in receipt should take the final state commitment")
					}

				}).
				Return(nil)

			// get the resources to use from the current test suite
			received := testcase.getResource(suite)
			err := eng.Process(testcase.from.NodeID, received)
			suite.Assert().Nil(err)

			// asserts verifier engine gets the call with a verifiable chunk
			suite.verifierEng.AssertExpectations(suite.T())

			// asserts the collection should not be requested
			suite.collectionsConduit.AssertNotCalled(suite.T(), "Submit", testifymock.Anything, collIdentity)
			// asserts the chunk state should not be requested
			suite.statesConduit.AssertNotCalled(suite.T(), "Submit", testifymock.Anything, execIdentity)
		})
	}
}

// TestChunkDataPackTracker_UntrackedChunkDataPack tests that ingest engine process method returns an error
// if it receives a ChunkDataPackResponse that does not have any tracker in the engine's mempool
func (suite *TestSuite) TestChunkDataPackTracker_UntrackedChunkDataPack() {
	// locks to run the tests sequentially
	suite.Lock()
	defer suite.Unlock()

	suite.SetupTest()
	eng := suite.TestNewEngine()

	execIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))

	// creates a chunk fixture, its data pack, and the data pack response
	chunk := unittest.ChunkFixture()
	chunkDataPack := unittest.ChunkDataPackFixture(chunk.ID())
	chunkDataPackResponse := &messages.ChunkDataPackResponse{Data: chunkDataPack}

	// mocks tracker to return an error for this chunk ID
	suite.chunkDataPackTrackers.On("ByChunkID", chunkDataPack.ChunkID).
		Return(nil, fmt.Errorf("does not exist"))
	// engine has not yet ingested this chunk
	suite.ingestedChunkIDs.On("Has", chunkDataPack.ChunkID).Return(false)

	err := eng.Process(execIdentity.NodeID, chunkDataPackResponse)

	// asserts that process of an untracked chunk data pack returns with an error
	suite.Assert().NotNil(err)
	suite.chunkDataPackTrackers.AssertExpectations(suite.T())
}

// TestChunkDataPackTracker_HappyPath evaluates the happy path of receiving a chunk data pack upon a request
func (suite *TestSuite) TestChunkDataPackTracker_HappyPath() {
	// locks to run the test sequentially
	suite.Lock()
	defer suite.Unlock()

	suite.SetupTest()
	eng := suite.TestNewEngine()

	execIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))

	// creates a block
	block := unittest.BlockFixture()
	// creates a chunk fixture, its data pack, and the data pack response for the block
	chunk := unittest.ChunkFixture()
	chunkDataPack := unittest.ChunkDataPackFixture(chunk.ID())
	chunkDataPackResponse := &messages.ChunkDataPackResponse{Data: chunkDataPack}

	// creates a tracker for chunk data pack that binds it to the block
	track := &tracker.ChunkDataPackTracker{
		BlockID: block.ID(),
		ChunkID: chunkDataPack.ChunkID,
	}

	// mocks tracker to return the tracker for the chunk data pack
	suite.chunkDataPackTrackers.On("ByChunkID", chunkDataPack.ChunkID).Return(track, nil).Once()

	// mocks state of ingest engine to return execution node ID
	suite.state.On("AtBlockID", track.BlockID).Return(suite.ss, nil).Once()
	suite.ss.On("Identity", execIdentity.NodeID).Return(execIdentity, nil).Once()

	// chunk data pack should be successfully added to mempool and the tracker should be removed
	suite.chunkDataPacks.On("Add", &chunkDataPack).Return(nil).Once()
	suite.chunkDataPackTrackers.On("Rem", chunkDataPack.ChunkID).Return(true).Once()

	// engine has not yet ingested this chunk
	suite.ingestedChunkIDs.On("Has", chunkDataPack.ChunkID).Return(false)
	// suite.ingestedChunkIDs.On("Add", chunk).Return(nil)

	// terminates call to checkPendingChunks as it is out of this test's scope
	suite.authReceipts.On("All").Return(nil).Once()

	err := eng.Process(execIdentity.NodeID, chunkDataPackResponse)

	// asserts that process of a tracked chunk data pack should return no error
	suite.Assert().Nil(err)
	suite.chunkDataPackTrackers.AssertExpectations(suite.T())
	suite.chunkDataPacks.AssertExpectations(suite.T())
	suite.state.AssertExpectations(suite.T())
	suite.ss.AssertExpectations(suite.T())
	suite.authReceipts.AssertExpectations(suite.T())
}
