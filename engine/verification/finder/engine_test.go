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

	// other engines
	// mock match engine, should be called when all dependent resources
	// for a receipt have been received by the finder engine.
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

// TestFinderEngine tests the establishment of the network registration upon
// creation of an instance of FinderEngine using the New method.
// It also returns an instance of new engine to be used in the later tests.
func (suite *FinderEngineTestSuite) TestFinderEngine() *finder.Engine {
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
