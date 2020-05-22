package ingest_test

import (
	"bytes"
	"math/rand"
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
	"github.com/dapperlabs/flow-go/model/verification/tracker"
	mempool "github.com/dapperlabs/flow-go/module/mempool/mock"
	module "github.com/dapperlabs/flow-go/module/mock"
	network "github.com/dapperlabs/flow-go/network/mock"
	protocol "github.com/dapperlabs/flow-go/state/protocol/mock"
	storage "github.com/dapperlabs/flow-go/storage/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// LightIngestTestSuite contains the context of a verifier engine test using mocked components.
type LightIngestTestSuite struct {
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
	receipts              *mempool.Receipts
	collections           *mempool.Collections
	chunkDataPacks        *mempool.ChunkDataPacks
	chunkDataPackTrackers *mempool.ChunkDataPackTrackers
	collectionTrackers    *mempool.CollectionTrackers
	ingestedChunkIDs      *mempool.Identifiers
	ingestedResultIDs     *mempool.Identifiers
	ingestedCollectionIDs *mempool.Identifiers
	assignedChunkIDs      *mempool.Identifiers
	headerStorage         *storage.Headers
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
	// identities
	verIdentity  *flow.Identity // verification node
	execIdentity *flow.Identity // execution node
	collIdentity *flow.Identity // collection node
}

// TestLightIngestEngine executes all LightIngestTestSuite tests.
func TestLightIngestEngine(t *testing.T) {
	suite.Run(t, new(LightIngestTestSuite))
}

// SetupTest initiates the test setups prior to each test.
func (suite *LightIngestTestSuite) SetupTest() {
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
	suite.headerStorage = &storage.Headers{}
	suite.blockStorage = &storage.Blocks{}
	suite.receipts = &mempool.Receipts{}
	suite.collections = &mempool.Collections{}
	suite.chunkDataPacks = &mempool.ChunkDataPacks{}
	suite.chunkDataPackTrackers = &mempool.ChunkDataPackTrackers{}
	suite.collectionTrackers = &mempool.CollectionTrackers{}
	suite.ingestedResultIDs = &mempool.Identifiers{}
	suite.ingestedChunkIDs = &mempool.Identifiers{}
	suite.assignedChunkIDs = &mempool.Identifiers{}
	suite.ingestedCollectionIDs = &mempool.Identifiers{}
	suite.assigner = &module.ChunkAssigner{}

	completeER := test.CompleteExecutionResultFixture(suite.T(), 1)
	suite.collection = completeER.Collections[0]
	suite.block = completeER.Block
	suite.receipt = completeER.Receipt
	suite.chunk = completeER.Receipt.ExecutionResult.Chunks[0]
	suite.chunkDataPack = completeER.ChunkDataPacks[0]
	suite.collTracker = tracker.NewCollectionTracker(suite.collection.ID(), suite.block.ID())
	suite.chunkTracker = tracker.NewChunkDataPackTracker(suite.chunk.ID(), suite.block.ID())

	suite.verIdentity = unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	suite.execIdentity = unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	suite.collIdentity = unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))

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

	suite.state.On("Final").Return(suite.ss)
	suite.state.On("AtBlockID", suite.block.ID()).Return(suite.ss, nil)

	// mocks identity of the verification node
	suite.me.On("NodeID").Return(suite.verIdentity.NodeID)

	// mocks chunk assignment
	//
	// assigns all chunks in the receipt to this node through mocking
	a := chmodel.NewAssignment()
	for _, chunk := range suite.receipt.ExecutionResult.Chunks {
		a.Add(chunk, []flow.Identifier{suite.verIdentity.NodeID})
	}
	suite.assigner.On("Assign",
		testifymock.Anything,
		testifymock.Anything,
		testifymock.Anything).
		Return(a, nil)
}

// TestNewLightEngine verifies the establishment of the network registration upon
// creation of an instance of LightIngestEngine using the New method
// It also returns an instance of new engine to be used in the later tests
func (suite *LightIngestTestSuite) TestNewLightEngine() *ingest.LightEngine {
	e, err := ingest.NewLightEngine(zerolog.Logger{},
		suite.net,
		suite.state,
		suite.me,
		suite.verifierEng,
		suite.receipts,
		suite.collections,
		suite.chunkDataPacks,
		suite.collectionTrackers,
		suite.chunkDataPackTrackers,
		suite.ingestedChunkIDs,
		suite.ingestedResultIDs,
		suite.ingestedCollectionIDs,
		suite.assignedChunkIDs,
		suite.headerStorage,
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
func (suite *LightIngestTestSuite) TestHandleBlock() {
	// locks to run the test sequentially
	suite.Lock()
	defer suite.Unlock()

	eng := suite.TestNewLightEngine()
	err := eng.Process(unittest.IdentifierFixture(), suite.block)
	assert.Equal(suite.T(), err, ingest.ErrInvType)
}

// TestHandleReceipt_MissingCollection evaluates that when the LightIngestEngine has both a receipt and its block
// but not the collections, it asks for the collections through the network.
func (suite *LightIngestTestSuite) TestHandleReceipt_MissingCollection() {
	// locks to run the test sequentially
	suite.Lock()
	defer suite.Unlock()

	eng := suite.TestNewLightEngine()

	// mocks state snapshot to return collIdentities as identity list of staked collection nodes
	suite.ss.On("Identities", testifymock.AnythingOfType("flow.IdentityFilter")).Return(flow.IdentityList{suite.collIdentity}, nil)

	// mocks existing resources at the engine's disposal
	//
	// mocks the possession of 'suite.block` in the storage
	suite.blockStorage.On("ByID", suite.block.ID()).Return(suite.block, nil)
	// mocks the possession of chunk data pack associated with the `suite.block`
	suite.chunkDataPacks.On("Has", suite.chunkDataPack.ID()).Return(true)
	suite.chunkDataPacks.On("ByChunkID", suite.chunkDataPack.ID()).Return(suite.chunkDataPack, true)

	// mocks missing collection
	//
	// mocks the absence of `suite.collection` which is the associated collection to this block
	// the collection does not exist in mempool
	suite.collections.On("ByID", suite.collection.ID()).Return(nil, false).Once()

	// engine has not yet ingested the result of this receipt yet
	suite.ingestedResultIDs.On("Has", suite.receipt.ExecutionResult.ID()).
		Return(false)
	suite.ingestedChunkIDs.On("Has", suite.chunk.ID()).
		Return(false)
	// mocks handling functionality
	suite.assignedChunkIDs.On("Add", suite.chunk.ID()).Return(true)
	suite.assignedChunkIDs.On("Has", suite.chunk.ID()).Return(true)

	// mocks functionalities of adding receipt and chunk to memory pools
	suite.receipts.On("Add", suite.receipt).Return(true).Once()

	// mocks trackers functionality for the chunk
	suite.collectionTrackers.On("Add", suite.collTracker).Return(true)
	suite.collectionTrackers.On("Has", suite.collection.ID()).Return(false)

	var submitWG sync.WaitGroup
	submitWG.Add(1)
	// expects a collection request is submitted
	suite.collectionsConduit.
		On("Submit", testifymock.AnythingOfType("*messages.CollectionRequest"), suite.collIdentity.NodeID).
		Run(func(args testifymock.Arguments) {
			submitWG.Done()
		}).Return(nil).Once()

	err := eng.Process(suite.execIdentity.NodeID, suite.receipt)
	suite.Assert().Nil(err)

	// starts engine
	<-eng.Ready()

	// starts timer for submitting request
	unittest.RequireReturnsBefore(suite.T(), submitWG.Wait,
		time.Duration(int64(suite.failureThreshold*suite.requestInterval))*time.Millisecond)

	// waits for the engine to get shutdown
	<-eng.Done()

	// asserts necessary calls
	suite.receipts.AssertExpectations(suite.T())
	suite.collectionsConduit.AssertExpectations(suite.T())

	// verifier should not be called
	suite.verifierEng.AssertNotCalled(suite.T(), "ProcessLocal", testifymock.Anything)
}

// TestHandleReceipt_MissingChunkDataPack evaluates that when the LightIngestEngine has both a receipt and its block
// but not the chunk data pack of it, it asks for the chunk data pack through the network
func (suite *LightIngestTestSuite) TestHandleReceipt_MissingChunkDataPack() {
	// locks to run the test sequentially
	suite.Lock()
	defer suite.Unlock()

	eng := suite.TestNewLightEngine()

	// mocks state snapshot to return exeIdentities as identity list of staked collection nodes
	suite.ss.On("Identities", testifymock.AnythingOfType("flow.IdentityFilter")).Return(flow.IdentityList{suite.execIdentity}, nil)

	// mocks existing resources at the engine's disposal
	//
	// block
	suite.blockStorage.On("ByID", suite.block.ID()).Return(suite.block, nil)
	// collection
	suite.collections.On("Has", suite.collection.ID()).Return(true)
	suite.collections.On("ByID", suite.collection.ID()).Return(suite.collection, true)

	// mocks missing resources
	//
	// absence of chunk data pack itself
	suite.chunkDataPacks.On("ByChunkID", suite.chunkDataPack.ID()).Return(nil, false)

	// engine has not yet ingested the result of this receipt as well as its chunks yet
	suite.ingestedResultIDs.On("Has", suite.receipt.ExecutionResult.ID()).Return(false)
	suite.ingestedChunkIDs.On("Has", suite.chunk.ID()).Return(false)

	// mocks functionalities
	//
	// adding functionality of chunk tracker to trackers mempool
	// mocks initial insertion of tracker into mempool
	suite.chunkDataPackTrackers.On("Add", suite.chunkTracker).Return(true)
	// mocks tracker check
	// absence of a tracker for chunk data pack
	suite.chunkDataPackTrackers.On("Has", suite.chunkDataPack.ID()).Return(false)
	// mocks the functionality of adding receipt to the mempool
	suite.receipts.On("Add", suite.receipt).Return(true).Once()
	// mocks handling functionality
	suite.assignedChunkIDs.On("Add", suite.chunk.ID()).Return(true)
	suite.assignedChunkIDs.On("Has", suite.chunk.ID()).Return(true)

	var submitWG sync.WaitGroup
	submitWG.Add(1)
	suite.chunksConduit.
		On("Submit", testifymock.AnythingOfType("*messages.ChunkDataPackRequest"),
			suite.execIdentity.NodeID).Run(func(args testifymock.Arguments) {
		submitWG.Done()
	}).Return(nil).Once()

	err := eng.Process(suite.execIdentity.NodeID, suite.receipt)
	suite.Assert().Nil(err)

	// starts engine
	<-eng.Ready()

	// starts timer for submitting request
	unittest.RequireReturnsBefore(suite.T(), submitWG.Wait,
		time.Duration(int64(suite.failureThreshold*suite.requestInterval))*time.Millisecond)

	// waits for the engine to get shutdown
	<-eng.Done()

	// asserts necessary calls
	suite.receipts.AssertExpectations(suite.T())
	suite.chunksConduit.AssertExpectations(suite.T())

	// verifier should not be called
	suite.verifierEng.AssertNotCalled(suite.T(), "ProcessLocal", testifymock.Anything)
}

// TestIngestedResult evaluates the happy path of submitting an execution receipt with an already ingested result
func (suite *LightIngestTestSuite) TestIngestedResult() {
	// locks to run the test sequentially
	suite.Lock()
	defer suite.Unlock()

	eng := suite.TestNewLightEngine()

	// mocks this receipt's result as ingested
	suite.ingestedResultIDs.On("Has", suite.receipt.ExecutionResult.ID()).Return(true)

	// nothing else is mocked, hence the process should simply return nil
	err := eng.Process(unittest.IdentifierFixture(), suite.receipt)
	require.NoError(suite.T(), err)
}

// TestIngestedChunk evaluates the happy path of submitting a chunk data pack for an already ingested chunk
func (suite *LightIngestTestSuite) TestIngestedChunk() {
	// locks to run the test sequentially
	suite.Lock()
	defer suite.Unlock()

	eng := suite.TestNewLightEngine()

	chunkDataPackResponse := &messages.ChunkDataResponse{
		ChunkDataPack: *suite.chunkDataPack,
		Nonce:         rand.Uint64(),
	}
	// mocks this chunk id
	suite.ingestedChunkIDs.On("Has", suite.chunkDataPack.ChunkID).Return(true)

	// nothing else is mocked, hence the process should simply return nil
	err := eng.Process(unittest.IdentifierFixture(), chunkDataPackResponse)
	require.NoError(suite.T(), err)

	// chunk data pack should not be tried to be stored in the mempool
	suite.chunkDataPacks.AssertNotCalled(suite.T(), "Add", testifymock.Anything)
}

// TestHandleCollection evaluates receiving a collection without any other receipt-dependent resources
// the collection should be added to the collection pool.
func (suite *LightIngestTestSuite) TestHandleCollection() {
	// locks to run the tests sequentially
	suite.Lock()
	defer suite.Unlock()

	eng := suite.TestNewLightEngine()

	suite.receipts.On("All").Return([]*flow.ExecutionReceipt{}, nil)
	suite.ss.On("Identity", suite.collIdentity.NodeID).Return(suite.collIdentity, nil)

	// expect that the collection be added to the mempool
	suite.collections.On("Add", suite.collection).Return(true).Once()

	// mocks collection has not been ingested
	suite.ingestedCollectionIDs.On("Has", suite.collection.ID()).Return(false)

	// mocks collection does not exist in authenticated and pending collections
	suite.collections.On("Has", suite.collection.ID()).Return(false).Once()

	// mocks collection tracker is cleaned up
	suite.collectionTrackers.On("Rem", suite.collection.ID()).Return(true).Once()

	err := eng.Process(suite.collIdentity.NodeID, suite.collection)
	suite.Assert().Nil(err)

	suite.collections.AssertExpectations(suite.T())

	// verifier should not be called
	suite.verifierEng.AssertNotCalled(suite.T(), "ProcessLocal", testifymock.Anything)
}

// TestHandleCollection_IngestedCollection evaluates the happy path of submitting a collection an already ingested chunk
func (suite *LightIngestTestSuite) TestHandleCollection_IngestedCollection() {
	// locks to run the test sequentially
	suite.Lock()
	defer suite.Unlock()

	eng := suite.TestNewLightEngine()

	// mocks this collection as ingested
	suite.ingestedCollectionIDs.On("Has", suite.collection.ID()).Return(true)

	// nothing else is mocked, hence the process should simply return nil
	err := eng.Process(unittest.IdentifierFixture(), suite.collection)
	require.NoError(suite.T(), err)

	// collection should not be tried to be stored in the mempool
	suite.collections.AssertNotCalled(suite.T(), "Add", testifymock.Anything)
}

// TestHandleCollection_ExistingAuthenticated evaluates the happy path of submitting a collection
// that currently exists in the mempool.
func (suite *LightIngestTestSuite) TestHandleCollection_Existing() {
	// locks to run the test sequentially
	suite.Lock()
	defer suite.Unlock()

	eng := suite.TestNewLightEngine()

	// mocks this collection as ingested
	suite.ingestedCollectionIDs.On("Has", suite.collection.ID()).Return(false)
	suite.collections.On("Has", suite.collection.ID()).Return(true)

	// nothing else is mocked, hence the process should simply return nil
	err := eng.Process(unittest.IdentifierFixture(), suite.collection)
	require.NoError(suite.T(), err)

	// collection should not be tried to be stored in the mempool
	suite.collections.AssertNotCalled(suite.T(), "Add", testifymock.Anything)
}

// TestVerifyReady evaluates that a verifiable chunk is locally passed to the verifier engine
// whenever all of its relevant resources are ready regardless of the order in which dependent resources are received.
func (suite *LightIngestTestSuite) TestVerifyReady() {
	testcases := []struct {
		getResource func(*LightIngestTestSuite) interface{}
		from        *flow.Identity
		label       string
	}{
		{
			getResource: func(s *LightIngestTestSuite) interface{} {
				// we assume collection exists in engine before the receipt arrives
				suite.collections.On("Has", suite.collection.ID()).Return(true)
				return s.receipt
			},
			from:  suite.execIdentity,
			label: "received receipt",
		},
		{
			getResource: func(s *LightIngestTestSuite) interface{} {
				// we assume the collection does not exist but already requested
				suite.collections.On("Has", suite.collection.ID()).Return(false)
				return s.collection
			},
			from:  suite.collIdentity,
			label: "received collection",
		},
	}

	for _, testcase := range testcases {
		suite.Run(testcase.label, func() {
			// locks to run the tests sequentially
			suite.Lock()
			defer suite.Unlock()

			suite.SetupTest()
			eng := suite.TestNewLightEngine()
			// mocks state
			suite.ss.On("Identities", testifymock.AnythingOfType("flow.IdentityFilter")).Return(flow.IdentityList{suite.verIdentity}, nil)

			// mocks the functionality of adding collection to the mempool
			suite.collections.On("Add", suite.collection).Return(true)

			// mocks missing resources
			//
			// engine has not yet ingested this receipt and its chunk
			suite.ingestedResultIDs.On("Has", suite.receipt.ExecutionResult.ID()).
				Return(false).Once()

			for _, chunk := range suite.receipt.ExecutionResult.Chunks {
				suite.ingestedChunkIDs.On("Has", chunk.ID()).Return(false)
				suite.ingestedChunkIDs.On("Add", chunk.ID()).Return(true)
			}

			// mocks available resources at engine's disposal
			//
			// block
			suite.blockStorage.On("ByID", suite.block.ID()).Return(suite.block, nil)
			// collection:
			suite.collections.On("ByID", suite.collection.ID()).Return(suite.collection, true)
			suite.ingestedCollectionIDs.On("Add", suite.collection.ID()).Return(true)
			suite.ingestedCollectionIDs.On("Has", suite.collection.ID()).Return(false)

			// chunk data pack in mempool
			suite.chunkDataPacks.On("Has", suite.chunkDataPack.ID()).Return(true)
			suite.chunkDataPacks.On("ByChunkID", suite.chunkDataPack.ID()).Return(suite.chunkDataPack, true)
			suite.assignedChunkIDs.On("Add", suite.chunk.ID()).Return(true)
			suite.assignedChunkIDs.On("Has", suite.chunk.ID()).Return(true)
			// execution receipt in authenticated pool
			suite.receipts.On("Add", suite.receipt).Return(true)
			suite.receipts.On("All").Return([]*flow.ExecutionReceipt{suite.receipt}, nil)

			// mocks cleanup functionalities
			//
			// mocks removing collection from authenticated collections
			suite.collections.On("Rem", suite.collection.ID()).Return(true)
			// mocks removing chunk data pack from the mempool
			suite.chunkDataPacks.On("Rem", suite.chunkDataPack.ID()).Return(true)
			// mocks removing ingested receipt
			suite.receipts.On("Rem", suite.receipt.ID()).Return(true)
			suite.collectionTrackers.On("Rem", suite.collection.ID()).Return(true)
			// mocks execution receipt is ingested literally
			suite.ingestedResultIDs.On("Add", suite.receipt.ExecutionResult.ID()).Return(true)

			// mocks test expectation
			//
			// verifier engine should get called locally by a verifiable chunk
			// also checks the end state of verifiable chunks about edge cases
			var receivedWG sync.WaitGroup
			receivedWG.Add(1)
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

					receivedWG.Done()

				}).
				Return(nil)

			// get the resources to use from the current test suite
			received := testcase.getResource(suite)
			err := eng.Process(testcase.from.NodeID, received)
			suite.Assert().Nil(err)

			// starts engine
			<-eng.Ready()

			unittest.RequireReturnsBefore(suite.T(), receivedWG.Wait,
				time.Duration(int64(suite.failureThreshold*suite.requestInterval))*time.Millisecond)

			// waits for the engine to get shutdown
			<-eng.Done()

			// asserts verifier engine gets the call with a verifiable chunk
			suite.verifierEng.AssertExpectations(suite.T())

			// asserts the collection should not be requested
			suite.collectionsConduit.AssertNotCalled(suite.T(), "Submit", testifymock.Anything, suite.collIdentity)
			// asserts the chunk state should not be requested
			suite.statesConduit.AssertNotCalled(suite.T(), "Submit", testifymock.Anything, suite.execIdentity)
		})
	}
}

// TestChunkDataPackTracker_HappyPath evaluates the happy path of receiving a chunk data pack upon a request
func (suite *LightIngestTestSuite) TestChunkDataPackTracker_HappyPath() {
	// locks to run the test sequentially
	suite.Lock()
	defer suite.Unlock()

	eng := suite.TestNewLightEngine()

	chunkDataPackResponse := &messages.ChunkDataResponse{
		ChunkDataPack: *suite.chunkDataPack,
		Nonce:         rand.Uint64(),
	}

	// engine should not already have the chunk data pack
	suite.chunkDataPacks.On("Has", suite.chunkDataPack.ChunkID).Return(false)
	// chunk data pack should be successfully added to mempool and the tracker should be removed
	suite.chunkDataPacks.On("Add", suite.chunkDataPack).Return(true).Once()
	suite.chunkDataPackTrackers.On("Rem", suite.chunkDataPack.ChunkID).Return(true).Once()

	// engine has not yet ingested this chunk
	suite.ingestedChunkIDs.On("Has", suite.chunkDataPack.ChunkID).Return(false).Once()

	// engine does not have any receipt
	suite.receipts.On("All").Return([]*flow.ExecutionReceipt{})

	err := eng.Process(suite.execIdentity.NodeID, chunkDataPackResponse)

	// asserts that process of a tracked chunk data pack should return no error
	suite.Assert().Nil(err)
	suite.chunkDataPackTrackers.AssertExpectations(suite.T())
	suite.chunkDataPacks.AssertExpectations(suite.T())
	suite.ingestedChunkIDs.AssertExpectations(suite.T())
}
