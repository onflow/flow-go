package ingest_test

import (
	"fmt"
	"testing"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/engine"
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
	authReceipts         *mempool.Receipts
	pendingReceipts      *mempool.PendingReceipts
	authCollections      *mempool.Collections
	pendingCollections   *mempool.PendingCollections
	collectionTrackers   *mempool.CollectionTrackers
	chunkDataPacks       *mempool.ChunkDataPacks
	chunkDataPackTracker *mempool.ChunkDataPackTrackers
	ingestedChunkIDs     *mempool.IngestedChunkIDs
	ingestedResultIDs    *mempool.IngestedResultIDs
	blockStorage         *storage.Blocks
	// resources fixtures
	collection    *flow.Collection
	block         *flow.Block
	receipt       *flow.ExecutionReceipt
	chunkDataPack *flow.ChunkDataPack
	assigner      *module.ChunkAssigner
	collTracker   *tracker.CollectionTracker

	// records unique chunk data pack requests to mock execution node
	reqChunksExe map[flow.Identifier]struct{}
}

// Invoking this method executes all TestSuite tests.
func TestReceiptsEngine(t *testing.T) {
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
	suite.chunkDataPackTracker = &mempool.ChunkDataPackTrackers{}
	suite.ingestedResultIDs = &mempool.IngestedResultIDs{}
	suite.ingestedChunkIDs = &mempool.IngestedChunkIDs{}
	suite.assigner = &module.ChunkAssigner{}

	completeER := test.CompleteExecutionResultFixture(suite.T(), 1)
	suite.collection = completeER.Collections[0]
	suite.block = completeER.Block
	suite.receipt = completeER.Receipt
	suite.chunkDataPack = completeER.ChunkDataPacks[0]
	suite.collTracker = &tracker.CollectionTracker{
		BlockID:      suite.block.ID(),
		CollectionID: suite.collection.ID(),
	}

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
		suite.chunkDataPackTracker,
		suite.ingestedChunkIDs,
		suite.ingestedResultIDs,
		suite.blockStorage,
		suite.assigner)
	require.Nil(suite.T(), err, "could not create an engine")

	suite.net.AssertExpectations(suite.T())

	return e
}

// TestHandleBlock passes a block to ingest engine and evaluates internal path
// as ingest engine only accepts a block through consensus follower, it should return an error
func (suite *TestSuite) TestHandleBlock() {
	eng := suite.TestNewEngine()
	err := eng.Process(unittest.IdentifierFixture(), suite.block)
	assert.Equal(suite.T(), err, ingest.ErrInvType)
}

// TestHandleReceipt_MissingCollection evaluates that when ingest engine has both a receipt and its block
// but not the collections, it asks for the collections through the network
func (suite *TestSuite) TestHandleReceipt_MissingCollection() {
	eng := suite.TestNewEngine()

	// mock the receipt coming from an execution node
	execIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	verIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	collIdentities := unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleCollection))

	suite.state.On("Final").Return(suite.ss, nil)
	suite.state.On("AtBlockID", testifymock.Anything).Return(suite.ss, nil).Once()
	suite.ss.On("Identity", execIdentity.NodeID).Return(execIdentity, nil).Once()
	suite.ss.On("Identities", testifymock.Anything).Return(collIdentities, nil).Twice()

	// we have the corresponding block and chunk state, but not the collection
	suite.blockStorage.On("ByID", suite.block.ID()).Return(suite.block, nil).Once()
	suite.authCollections.On("Has", suite.collection.ID()).Return(false).Once()

	// mocks that collection does not exit in the mempools and we do not have a tracker for it
	suite.pendingCollections.On("Has", suite.collection.ID()).Return(false).Once()
	suite.collectionTrackers.On("Has", suite.collection.ID()).Return(false).Once()
	suite.collectionTrackers.On("Add", suite.collTracker).Return(nil).Once()

	suite.chunkDataPacks.On("Has", suite.chunkDataPack.ID()).Return(true).Once()
	suite.chunkDataPacks.On("ByChunkID", suite.chunkDataPack.ID()).Return(suite.chunkDataPack, nil).Once()

	// engine has not yet ingested the result of this receipt yet
	suite.ingestedResultIDs.On("Has", suite.receipt.ExecutionResult.ID()).Return(false)

	for _, chunk := range suite.receipt.ExecutionResult.Chunks {
		suite.ingestedChunkIDs.On("Has", chunk.ID()).Return(false)
	}

	// expect that we already have the receipt in the authenticated receipts mempool
	suite.authReceipts.On("All").Return([]*flow.ExecutionReceipt{suite.receipt}, nil).Once()
	suite.pendingReceipts.On("All").Return([]*verificationmodel.PendingReceipt{}).Once()
	suite.authReceipts.On("Add", suite.receipt).Return(nil).Once()

	// expect that the collection is requested
	suite.collectionsConduit.On("Submit", testifymock.Anything, collIdentities[0].NodeID).Return(nil).Once()

	// assigns all chunks in the receipt to this node through mocking
	a := chmodel.NewAssignment()
	for _, chunk := range suite.receipt.ExecutionResult.Chunks {
		a.Add(chunk, []flow.Identifier{verIdentity.NodeID})
	}
	suite.assigner.On("Assign",
		testifymock.Anything,
		testifymock.Anything,
		testifymock.Anything).Return(a, nil)
	suite.me.On("NodeID", testifymock.Anything).Return(verIdentity.NodeID)

	err := eng.Process(execIdentity.NodeID, suite.receipt)
	suite.Assert().Nil(err)

	suite.authReceipts.AssertExpectations(suite.T())
	suite.collectionsConduit.AssertExpectations(suite.T())

	// verifier should not be called
	suite.verifierEng.AssertNotCalled(suite.T(), "ProcessLocal", testifymock.Anything)
}

// TestIngestedResult evaluates the happy path of submitting an execution receipt with an already ingested result
func (suite *TestSuite) TestIngestedResult() {
	eng := suite.TestNewEngine()

	// mocks this receipt's result as ingested
	suite.ingestedResultIDs.On("Has", suite.receipt.ExecutionResult.ID()).Return(true)

	// nothing else is mocked, hence the process should simply return nil
	err := eng.Process(unittest.IdentifierFixture(), suite.receipt)
	require.NoError(suite.T(), err)
}

// TestIngestedChunk evaluates the happy path of submitting a chunk data pack for an already ingested chunk
func (suite *TestSuite) TestIngestedChunk() {
	eng := suite.TestNewEngine()

	// mocks this chunk id
	suite.ingestedChunkIDs.On("Has", suite.chunkDataPack.ChunkID).Return(true)

	// nothing else is mocked, hence the process should simply return nil
	err := eng.Process(unittest.IdentifierFixture(), suite.chunkDataPack)
	require.NoError(suite.T(), err)
}

// TestHandleReceipt_UnstakedSender evaluates sending an execution receipt from an unstaked node
// it should go to the pending receipts and (later on) dropped from the cache
// Todo dropping unauthenticated receipts from cache
// https://github.com/dapperlabs/flow-go/issues/2966
func (suite *TestSuite) TestHandleReceipt_UnstakedSender() {
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
	eng := suite.TestNewEngine()

	// mock the collection coming from an collection node
	collIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))

	suite.authReceipts.On("All").Return([]*flow.ExecutionReceipt{}, nil)
	suite.pendingReceipts.On("All").Return([]*verificationmodel.PendingReceipt{}, nil)
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
	}
}

// TestVerifyReady evaluates that the verifier engine should be called
// when the receipt is ready regardless of
// the order in which dependent resources are received.
func (suite *TestSuite) TestVerifyReady() {

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
			suite.SetupTest()
			eng := suite.TestNewEngine()

			suite.state.On("Final", testifymock.Anything).Return(suite.ss, nil)
			suite.state.On("AtBlockID", testifymock.Anything).Return(suite.ss, nil)
			suite.ss.On("Identity", testcase.from.NodeID).Return(testcase.from, nil)
			suite.ss.On("Identities", testifymock.Anything).Return(flow.IdentityList{verIdentity}, nil)
			suite.me.On("NodeID").Return(verIdentity.NodeID)

			// allow adding the received resource to mempool
			suite.authCollections.On("Add", suite.collection).Return(nil)

			// engine has not yet ingested this receipt and chunk
			suite.ingestedResultIDs.On("Has", suite.receipt.ExecutionResult.ID()).Return(false).Once()

			for _, chunk := range suite.receipt.ExecutionResult.Chunks {
				suite.ingestedChunkIDs.On("Has", chunk.ID()).Return(false)
				suite.ingestedChunkIDs.On("Add", chunk).Return(nil)
			}

			// we have all dependencies
			suite.blockStorage.On("ByID", suite.block.ID()).Return(suite.block, nil)

			suite.authCollections.On("Has", suite.collection.ID()).Return(true)
			suite.authCollections.On("ByID", suite.collection.ID()).Return(suite.collection, nil)
			suite.authCollections.On("Rem", suite.collection.ID()).Return(true)

			suite.collectionTrackers.On("ByCollectionID", suite.collection.ID()).Return(suite.collTracker, nil)
			suite.collectionTrackers.On("Rem", suite.collection.ID()).Return(true)

			suite.chunkDataPacks.On("Has", suite.chunkDataPack.ID()).Return(true)
			suite.chunkDataPacks.On("Rem", suite.chunkDataPack.ID()).Return(true)

			suite.authReceipts.On("Add", suite.receipt).Return(nil)
			suite.authReceipts.On("All").Return([]*flow.ExecutionReceipt{suite.receipt}, nil)

			// creates and mocks a pending receipt for the testcase
			p := &verificationmodel.PendingReceipt{
				Receipt:  suite.receipt,
				OriginID: testcase.from.NodeID,
			}
			preceipts := []*verificationmodel.PendingReceipt{p}

			// receipt should go to the pending receipts mempool
			suite.pendingReceipts.On("All").Return(preceipts)
			suite.pendingReceipts.On("Rem", suite.receipt.ID()).Return(true)
			suite.chunkDataPacks.On("ByChunkID", suite.chunkDataPack.ID()).Return(suite.chunkDataPack, nil)

			// we have the assignment of chunk
			a := chmodel.NewAssignment()
			chunk, ok := suite.receipt.ExecutionResult.Chunks.ByIndex(0)
			require.True(suite.T(), ok, "chunk out of range requested")
			a.Add(chunk, flow.IdentifierList{verIdentity.NodeID})
			suite.assigner.On("Assign",
				testifymock.Anything,
				testifymock.Anything,
				testifymock.Anything).Return(a, nil)

			// we should call the verifier engine, as the receipt is ready for verification
			suite.verifierEng.On("ProcessLocal", testifymock.Anything).Return(nil).Once()

			// get the resource to use from the current test suite
			received := testcase.getResource(suite)
			err := eng.Process(testcase.from.NodeID, received)
			suite.Assert().Nil(err)

			suite.verifierEng.AssertExpectations(suite.T())

			// the collection should not be requested
			suite.collectionsConduit.AssertNotCalled(suite.T(), "Submit", testifymock.Anything, collIdentity)
			// the chunk state should not be requested
			suite.statesConduit.AssertNotCalled(suite.T(), "Submit", testifymock.Anything, execIdentity)

		})
	}
}

// TestChunkDataPackTracker_UntrackedChunkDataPack tests that ingest engine process method returns an error
// if it receives a ChunkDataPackResponse that does not have any tracker in the engine's mempool
func (suite *TestSuite) TestChunkDataPackTracker_UntrackedChunkDataPack() {
	suite.SetupTest()
	eng := suite.TestNewEngine()

	execIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))

	// creates a chunk fixture, its data pack, and the data pack response
	chunk := unittest.ChunkFixture()
	chunkDataPack := unittest.ChunkDataPackFixture(chunk.ID())
	chunkDataPackResponse := &messages.ChunkDataPackResponse{Data: chunkDataPack}

	// mocks tracker to return an error for this chunk ID
	suite.chunkDataPackTracker.On("ByChunkID", chunkDataPack.ChunkID).
		Return(nil, fmt.Errorf("does not exist"))
	// engine has not yet ingested this chunk
	suite.ingestedChunkIDs.On("Has", chunkDataPack.ChunkID).Return(false)

	err := eng.Process(execIdentity.NodeID, chunkDataPackResponse)

	// asserts that process of an untracked chunk data pack returns with an error
	suite.Assert().NotNil(err)
	suite.chunkDataPackTracker.AssertExpectations(suite.T())
}

// TestChunkDataPackTracker_HappyPath evaluates the happy path of receiving a chunk data pack upon a request
func (suite *TestSuite) TestChunkDataPackTracker_HappyPath() {
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
	suite.chunkDataPackTracker.On("ByChunkID", chunkDataPack.ChunkID).Return(track, nil).Once()

	// mocks state of ingest engine to return execution node ID
	suite.state.On("AtBlockID", track.BlockID).Return(suite.ss, nil).Once()
	suite.ss.On("Identity", execIdentity.NodeID).Return(execIdentity, nil).Once()

	// chunk data pack should be successfully added to mempool and the tracker should be removed
	suite.chunkDataPacks.On("Add", &chunkDataPack).Return(nil).Once()
	suite.chunkDataPackTracker.On("Rem", chunkDataPack.ChunkID).Return(true).Once()

	// engine has not yet ingested this chunk
	suite.ingestedChunkIDs.On("Has", chunkDataPack.ChunkID).Return(false)
	// suite.ingestedChunkIDs.On("Add", chunk).Return(nil)

	// terminates call to checkPendingChunks as it is out of this test's scope
	suite.authReceipts.On("All").Return(nil).Once()

	err := eng.Process(execIdentity.NodeID, chunkDataPackResponse)

	// asserts that process of a tracked chunk data pack should return no error
	suite.Assert().Nil(err)
	suite.chunkDataPackTracker.AssertExpectations(suite.T())
	suite.chunkDataPacks.AssertExpectations(suite.T())
	suite.state.AssertExpectations(suite.T())
	suite.ss.AssertExpectations(suite.T())
	suite.authReceipts.AssertExpectations(suite.T())
}
