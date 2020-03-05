package proposal

import (
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/consensus/coldstuff"
	"github.com/dapperlabs/flow-go/model/flow"
	mempool "github.com/dapperlabs/flow-go/module/mempool/mock"
	module "github.com/dapperlabs/flow-go/module/mock"
	"github.com/dapperlabs/flow-go/module/trace"
	network "github.com/dapperlabs/flow-go/network/mock"
	"github.com/dapperlabs/flow-go/network/stub"
	protocol "github.com/dapperlabs/flow-go/protocol/mock"
	storage "github.com/dapperlabs/flow-go/storage/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

type Suite struct {
	suite.Suite

	state        *protocol.State
	snapshot     *protocol.Snapshot
	me           *module.Local
	net          *stub.Network
	provider     *network.Engine
	pool         *mempool.Transactions
	transactions *storage.Transactions
	headers      *storage.Headers
	payloads     *storage.ClusterPayloads
	builder      *module.Builder
	finalizer    *module.Finalizer
}

func TestProposalEngine(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) SetupTest() {
	log := zerolog.New(os.Stderr)
	tracer, err := trace.NewTracer(log)
	require.NoError(suite.T(), err)

	suite.state = new(protocol.State)
	suite.snapshot = new(protocol.Snapshot)
	suite.state.On("Final").Return(suite.snapshot)
	suite.snapshot.On("Head").Return(&flow.Header{}, nil)
	suite.snapshot.On("Identities", mock.Anything).Return(unittest.IdentityListFixture(1), nil)
	suite.me = new(module.Local)
	suite.me.On("NodeID").Return(flow.Identifier{})

	hub := stub.NewNetworkHub()
	suite.net = stub.NewNetwork(suite.state, suite.me, hub)

	suite.provider = new(network.Engine)
	suite.pool = new(mempool.Transactions)
	suite.transactions = new(storage.Transactions)
	suite.headers = new(storage.Headers)
	suite.payloads = new(storage.ClusterPayloads)
	suite.builder = new(module.Builder)
	suite.finalizer = new(module.Finalizer)

	eng, err := New(log, suite.net, suite.me, suite.state, tracer, suite.provider, suite.pool, suite.transactions, suite.headers, suite.payloads)
	require.NoError(suite.T(), err)

	cold, err := coldstuff.New(log, suite.state, suite.me, eng, suite.builder, suite.finalizer, time.Second, time.Second)
	require.NoError(suite.T(), err)

	eng.coldstuff = cold
}

func (suite *Suite) TestHandleProposal() {}

func (suite *Suite) TestHandlePendingProposal() {}

func (suite *Suite) TestHandleProposalWithPendingChildren() {}

func (suite *Suite) TestReceiveVote() {}
