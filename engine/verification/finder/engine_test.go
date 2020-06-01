package finder_test

import (
	"testing"

	"github.com/rs/zerolog"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/verification/finder"
	"github.com/dapperlabs/flow-go/engine/verification/utils"
	"github.com/dapperlabs/flow-go/model/flow"
	mempool "github.com/dapperlabs/flow-go/module/mempool/mock"
	module "github.com/dapperlabs/flow-go/module/mock"
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

	// mock mempool for receipts
	receipts *mempool.Receipts

	// mock mempool for header storage of blocks
	headerStorage *storage.Headers
	// resources fixtures
	collection    *flow.Collection
	block         *flow.Block
	receipt       *flow.ExecutionReceipt
	chunk         *flow.Chunk
	chunkDataPack *flow.ChunkDataPack

	// identities
	verIdentity  *flow.Identity // verification node
	execIdentity *flow.Identity // execution node

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
	suite.headerStorage = &storage.Headers{}
	suite.receipts = &mempool.Receipts{}

	// generates an execution result with a single collection, chunk, and transaction.
	completeER := utils.LightExecutionResultFixture(1)
	suite.collection = completeER.Collections[0]
	suite.block = completeER.Block
	suite.receipt = completeER.Receipt
	suite.chunk = completeER.Receipt.ExecutionResult.Chunks[0]
	suite.chunkDataPack = completeER.ChunkDataPacks[0]

	suite.verIdentity = unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
	suite.execIdentity = unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))

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
		suite.net,
		suite.me,
		suite.matchEng,
		suite.receipts,
		suite.headerStorage)
	require.Nil(suite.T(), err, "could not create finder engine")

	suite.net.AssertExpectations(suite.T())

	return e
}

// TestFinderEngine_HappyPath evaluates that handling a receipt that is not duplicate,
// and its result has not processed yet ends by adding to receipt mempool.
func (suite *FinderEngineTestSuite) TestHandleReceipt_HappyPath() {
	e := suite.TestNewFinderEngine()

	// mocks this receipt is not in the receipts mempool
	suite.receipts.On("Has", suite.receipt.ID()).Return(false).Once()

	// mocks adding receipt to the receipts mempool
	suite.receipts.On("Add", suite.receipt).Return(true).Once()

	// sends receipt to finder engine
	err := e.Process(suite.execIdentity.NodeID, suite.receipt)
	require.NoError(suite.T(), err)

	suite.receipts.AssertExpectations(suite.T())
}

// TestHandleReceipt_Duplicate evaluates that handling a receipt that is duplicate,
// ends up dropping that receipt.
func (suite *FinderEngineTestSuite) TestHandleReceipt_Duplicate() {
	e := suite.TestNewFinderEngine()

	// mocks this receipt is already in the receipts mempool
	suite.receipts.On("Has", suite.receipt.ID()).Return(true).Once()

	// sends receipt to finder engine
	err := e.Process(suite.execIdentity.NodeID, suite.receipt)
	require.NoError(suite.T(), err)

	// duplicate receipt should not be added to the mempool
	suite.receipts.AssertNotCalled(suite.T(), "Add", suite.receipt)

	suite.receipts.AssertExpectations(suite.T())
}
