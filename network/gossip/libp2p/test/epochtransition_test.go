package test

import (
	"os"
	"sort"
	"testing"
	"time"

	golog "github.com/ipfs/go-log"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	mock2 "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/flow/order"
	"github.com/onflow/flow-go/network/gossip/libp2p"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// total number of nodes in our network
const nodeCount = 11

type EpochTransitionTestSuite struct {
	suite.Suite
	ConduitWrapper                      // used as a wrapper around conduit methods
	nets           []*libp2p.Network    // used to keep track of the networks
	mws            []*libp2p.Middleware // used to keep track of the middlewares associated with networks
	state          *protocol.State
	snapshot       *protocol.Snapshot
	epochQuery     *protocol.EpochQuery
	clusterList    flow.ClusterList
	ids            flow.IdentityList
	collectors     flow.IdentityList
}

func TestEpochTransitionTestSuite(t *testing.T) {
	suite.Run(t, new(EchoEngineTestSuite))
}

func (ts *EpochTransitionTestSuite) SetupTest() {
	ts.state = new(protocol.State)
	ts.snapshot = new(protocol.Snapshot)
	ts.epochQuery = new(protocol.EpochQuery)
	nClusters := 3
	nCollectors := 7
	ts.collectors = unittest.IdentityListFixture(nCollectors, unittest.WithRole(flow.RoleCollection))
	ts.ids = append(unittest.IdentityListFixture(1000, unittest.WithAllRolesExcept(flow.RoleCollection)), ts.collectors...)
	assignments := unittest.ClusterAssignment(uint(nClusters), ts.collectors)
	clusters, err := flow.NewClusterList(assignments, ts.collectors)
	require.NoError(ts.T(), err)
	ts.clusterList = clusters
	epoch := new(protocol.Epoch)
	epoch.On("Clustering").Return(clusters, nil).Times(nCollectors)

	ts.epochQuery.On("Current").Return(epoch).Times(nCollectors)
	ts.snapshot.On("Epochs").Return(ts.epochQuery).Times(nCollectors)
	ts.snapshot.On("Identities", mock2.Anything).Return(ts.ids, nil)
	ts.state.On("Final").Return(ts.snapshot, nil).Times(nCollectors)

	golog.SetAllLoggers(golog.LevelInfo)

	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()

	// create middleware
	mws, err := createMiddleware(logger, ts.ids)
	require.NoError(ts.T(), err)
	ts.mws = mws

	// create networks
	nets, err := createNetworks(logger, ts.mws, ts.ids, 100, false, nil, nil)
	require.NoError(ts.Suite.T(), err)
	ts.nets = nets
}

// TearDownTest closes the networks within a specified timeout
func (ts *EpochTransitionTestSuite) TearDownTest() {
	for _, net := range ts.nets {
		select {
		// closes the network
		case <-net.Done():
			continue
		case <-time.After(3 * time.Second):
			ts.Suite.Fail("could not stop the network")
		}
	}
}

func stateSnapshot(ids flow.IdentityList) *protocol.ReadOnlyState {
	state := new(protocol.ReadOnlyState)
	snapshot := new(protocol.Snapshot)
	state.On("Final").Return(snapshot, nil)
	snapshot.On("Identities", mock2.Anything).Return(ids, nil)
	epochQuery := new(protocol.EpochQuery)
	nClusters := 3

	return state
}
