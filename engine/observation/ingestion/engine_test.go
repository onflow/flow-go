package ingestion

import (
	"fmt"
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"

	module "github.com/dapperlabs/flow-go/module/mock"
	"github.com/dapperlabs/flow-go/module/trace"
	network "github.com/dapperlabs/flow-go/network/mock"
	protocol "github.com/dapperlabs/flow-go/protocol/mock"
	realstore "github.com/dapperlabs/flow-go/storage"
	storage "github.com/dapperlabs/flow-go/storage/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

type Suite struct {
	suite.Suite

	// protocol state
	proto struct {
		state    *protocol.State
		snapshot *protocol.Snapshot
		mutator  *protocol.Mutator
	}

	me           *module.Local
	net          *module.Network
	provider     *network.Engine
	blocks       *storage.Blocks
	headers      *storage.Headers
	collections  *storage.Collections
	transactions *storage.Transactions
	eng          *Engine

	// mock conduit for requesting/receiving collections
	collectionsConduit *network.Conduit
}

func TestIngestEngine(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) SetupTest() {
	log := zerolog.New(os.Stderr)
	tracer, err := trace.NewTracer(log)
	require.NoError(suite.T(), err)

	obsIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleObservation))

	// mock out protocol state
	suite.proto.state = new(protocol.State)
	suite.proto.snapshot = new(protocol.Snapshot)
	suite.proto.state.On("Identity").Return(obsIdentity, nil)
	suite.proto.state.On("Final").Return(suite.proto.snapshot, nil)

	suite.me = new(module.Local)
	suite.me.On("NodeID").Return(obsIdentity.NodeID)

	suite.net = new(module.Network)
	suite.collectionsConduit = &network.Conduit{}
	suite.net.On("Register", uint8(engine.CollectionProvider), mock.Anything).
		Return(suite.collectionsConduit, nil).
		Once()

	suite.provider = new(network.Engine)
	suite.blocks = new(storage.Blocks)
	suite.headers = new(storage.Headers)
	suite.collections = new(storage.Collections)
	suite.transactions = new(storage.Transactions)

	eng, err := New(log, suite.net, suite.proto.state, tracer, suite.me, suite.blocks, suite.headers, suite.collections, suite.transactions)
	require.NoError(suite.T(), err)
	suite.eng = eng

}

// TestHandleBlock checks that when a block is received, a request for each individual collection is made
func (suite *Suite) TestHandleBlock() {
	originID := unittest.IdentifierFixture()
	block := unittest.BlockFixture()

	cNodeIdentities := unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleCollection))
	suite.proto.snapshot.On("Identities", mock.Anything).Return(cNodeIdentities, nil).Once()

	// expect that the block storage is indexed with each of the collection guarantee
	suite.blocks.On("IndexByGuarantees", block.ID()).Return(nil).Once()

	// expect that the collection is requested
	suite.collectionsConduit.On("Submit", mock.Anything, mock.Anything).Return(nil).Times(len(block.Guarantees))

	err := suite.eng.Process(originID, &block)
	require.NoError(suite.T(), err)
	suite.proto.snapshot.AssertExpectations(suite.T())
	suite.headers.AssertExpectations(suite.T())
	suite.collectionsConduit.AssertExpectations(suite.T())
}

// TestHandleCollection checks that when a Collection is received, it is persisted
func (suite *Suite) TestHandleCollection() {
	originID := unittest.IdentifierFixture()
	collection := unittest.CollectionFixture(5)
	light := collection.Light()

	suite.collections.On("StoreLightAndIndexByTransaction", &light).Return(nil).Once()
	suite.transactions.On("Store", mock.Anything).Return(nil).Times(len(collection.Transactions))

	cr := messages.CollectionResponse{Collection: collection}
	err := suite.eng.Process(originID, &cr)

	require.NoError(suite.T(), err)
	suite.collections.AssertExpectations(suite.T())
	suite.transactions.AssertExpectations(suite.T())
}

// TestHandleDuplicateCollection checks that when a duplicate Collection is received, it is ignored
func (suite *Suite) TestHandleDuplicateCollection() {
	originID := unittest.IdentifierFixture()
	collection := unittest.CollectionFixture(5)
	light := collection.Light()

	error := fmt.Errorf("extra text: %w", realstore.ErrAlreadyExists)
	suite.collections.On("StoreLightAndIndexByTransaction", &light).Return(error).Once()

	cr := messages.CollectionResponse{Collection: collection}
	err := suite.eng.Process(originID, &cr)

	require.NoError(suite.T(), err)
	suite.collections.AssertExpectations(suite.T())
}
