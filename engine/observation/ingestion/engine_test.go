package ingestion

import (
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/observation"
	"github.com/dapperlabs/flow-go/model/flow"

	module "github.com/dapperlabs/flow-go/module/mock"
	"github.com/dapperlabs/flow-go/module/trace"
	network "github.com/dapperlabs/flow-go/network/mock"
	protocol "github.com/dapperlabs/flow-go/protocol/mock"
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
	transactions *storage.Transactions
	collections  *storage.Collections
	headers      *storage.Headers
	payloads     *storage.Payloads
	blkState     *observation.BlockchainState
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
	suite.transactions = new(storage.Transactions)
	suite.collections = new(storage.Collections)
	suite.headers = new(storage.Headers)
	suite.payloads = new(storage.Payloads)
	suite.blkState = observation.NewBlockchainState(suite.headers, suite.payloads, suite.collections, suite.transactions)

	eng, err := New(log, suite.net, suite.proto.state, tracer, suite.me, suite.blkState)
	require.NoError(suite.T(), err)
	suite.eng = eng

}

// TestHandleBlock checks that when a block is received a request for each individual collection is made
func (suite *Suite) TestHandleBlock() {
	originID := unittest.IdentifierFixture()
	block := unittest.BlockFixture()

	suite.collections.On("Store", mock.Anything).Return(nil).Times(len(block.Guarantees))

	collIdentities := unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleCollection))
	suite.proto.snapshot.On("Identities", mock.Anything).Return(collIdentities, nil).Once()

	// expect that the collection is requested
	suite.collectionsConduit.On("Submit", mock.Anything, mock.Anything).Return(nil).Times(len(block.Guarantees))

	err := suite.eng.Process(originID, &block)
	require.NoError(suite.T(), err)
	suite.net.AssertExpectations(suite.T())
}
