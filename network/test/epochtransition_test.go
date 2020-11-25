package test

import (
	"fmt"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"

	golog "github.com/ipfs/go-log"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/network"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// MutableIdentityTableSuite tests that the networking layer responds correctly
// to changes to the identity table. When nodes are added, we should update our
// topology and accept connections from these new nodes. When nodes are removed
// or ejected we should update our topology and restrict connections from these
// nodes.
type MutableIdentityTableSuite struct {
	suite.Suite
	ConduitWrapper
	nets         []*network.Network
	mws          []*network.Middleware
	idRefreshers []*network.NodeIDRefresher
	engines      []*MeshEngine
	state        *protocol.ReadOnlyState
	snapshot     *protocol.Snapshot
	ids          flow.IdentityList
	logger       zerolog.Logger
}

func TestEpochTransitionTestSuite(t *testing.T) {
	suite.Run(t, new(MutableIdentityTableSuite))
}

func (suite *MutableIdentityTableSuite) SetupTest() {
	rand.Seed(time.Now().UnixNano())
	nodeCount := 10
	suite.logger = zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
	golog.SetAllLoggers(golog.LevelError)

	// create ids
	ids, mws := GenerateIDsAndMiddlewares(suite.T(), nodeCount, !DryRun, suite.logger)
	suite.ids = ids
	suite.mws = mws

	// setup state related mocks
	final := unittest.BlockHeaderFixture()
	suite.state = new(protocol.ReadOnlyState)
	suite.snapshot = new(protocol.Snapshot)
	suite.snapshot.On("Head").Return(&final, nil)
	suite.snapshot.On("Phase").Return(flow.EpochPhaseCommitted, nil)
	suite.snapshot.On("Identities", testifymock.Anything).Return(
		func(flow.IdentityFilter) flow.IdentityList { return suite.ids },
		func(flow.IdentityFilter) error { return nil })
	suite.state.On("Final").Return(suite.snapshot, nil)

	// all nodes use the same state mock
	states := make([]*protocol.ReadOnlyState, nodeCount)
	for i := 0; i < nodeCount; i++ {
		states[i] = suite.state
	}

	// create networks using the mocked state and default topology
	sms := GenerateSubscriptionManagers(suite.T(), mws)
	nets := GenerateNetworks(suite.T(), suite.logger, ids, mws, 100, nil, sms, !DryRun)
	suite.nets = nets

	// generate the refreshers
	suite.idRefreshers = suite.generateNodeIDRefreshers(nets)

	// generate the engines
	suite.engines = GenerateEngines(suite.T(), nets)
}

// TearDownTest closes the networks within a specified timeout
func (suite *MutableIdentityTableSuite) TearDownTest() {
	stopNetworks(suite.T(), suite.nets, 3*time.Second)
}

// TestNewNodeAdded tests that when a new node is added to the identity list
// (ie. as a result of a EpochSetup event) that it can connect to the network.
func (suite *MutableIdentityTableSuite) TestNewNodeAdded() {

	// create the id, middleware and network for a new node
	ids, mws, nets := GenerateIDsMiddlewaresNetworks(suite.T(), 1, suite.logger, 100, nil, !DryRun)
	newID := ids[0]
	suite.nets = append(suite.nets, nets[0])
	newMiddleware := mws[0]

	newIDs := append(suite.ids, ids...)
	suite.ids = newIDs

	// create a new refresher
	newIDRefresher := suite.generateNodeIDRefreshers(nets)
	newIDRefreshers := append(suite.idRefreshers, newIDRefresher...)

	// create the engine for the new node
	newEngine := GenerateEngines(suite.T(), nets)
	newEngines := append(suite.engines, newEngine...)

	// trigger the identity table change event
	for _, n := range newIDRefreshers {
		n.OnIdentityTableChanged()
	}

	// check if the new node has sufficient connections with the existing nodes
	// if it does, then it has been inducted successfully in the network
	checkConnectivity(suite.T(), newMiddleware, newIDs.Filter(filter.Not(filter.HasNodeID(newID.NodeID))))

	// check that all the engines on this new epoch can talk to each other
	sendMessagesAndVerify(suite.T(), newIDs, newEngines, suite.Publish)
}

// TestNodeRemoved tests that when an existing node is removed from the identity
// list (ie. as a result of an ejection or transition into an epoch where that node
// has un-staked) that it cannot connect to the network.
func (suite *MutableIdentityTableSuite) TestNodeRemoved() {

	// choose a random node to remove
	removeIndex := rand.Intn(len(suite.ids))
	removedID := suite.ids[removeIndex]

	// remove the identity at that index from the ids
	newIDs := suite.ids.Filter(filter.Not(filter.HasNodeID(removedID.NodeID)))
	suite.ids = newIDs

	// create a list of engines except for the removed node
	var newEngines []*MeshEngine
	for i, eng := range suite.engines {
		if i == removeIndex {
			continue
		}
		newEngines = append(newEngines, eng)
	}

	// trigger an epoch phase change for all nodes
	// from flow.EpochPhaseStaking to flow.EpochPhaseSetup
	for _, n := range suite.idRefreshers {
		n.OnIdentityTableChanged()
	}

	// check that all remaining engines can still talk to each other
	sendMessagesAndVerify(suite.T(), newIDs, newEngines, suite.Publish)

	// TODO check that messages to/from evicted node are not delivered
}

// checkConnectivity checks that the middleware of a node is directly connected
// to at least half of the other nodes.
func checkConnectivity(t *testing.T, mw *network.Middleware, ids flow.IdentityList) {
	threshold := len(ids) / 2
	assert.Eventually(t, func() bool {
		connections := 0
		for _, id := range ids {
			connected, err := mw.IsConnected(*id)
			require.NoError(t, err)
			if connected {
				connections++
			}
		}
		return connections >= threshold
	}, 5*time.Second, time.Millisecond*100)
}

// sendMessagesAndVerify sends a message from each engine to the other engines
// and verifies that all the messages are delivered.
func sendMessagesAndVerify(t *testing.T, ids flow.IdentityList, engs []*MeshEngine, send ConduitSendWrapperFunc) {

	// allows nodes to find each other in case of Multicast and Publish
	optionalSleep(send)

	count := len(engs)
	expectedMsgCnt := count - 1

	// each node broadcasting a message to all others
	for i, eng := range engs {
		event := &message.TestMessage{
			Text: fmt.Sprintf("hello from node %d", i),
		}
		others := ids.Filter(filter.Not(filter.HasNodeID(ids[i].NodeID))).NodeIDs()
		require.NoError(t, send(event, eng.con, others...))
	}

	wg := sync.WaitGroup{}
	// fires a goroutine for each engine to listen to incoming messages
	for i := range engs {
		wg.Add(expectedMsgCnt)
		go func(e *MeshEngine) {
			for x := 0; x < expectedMsgCnt; x++ {
				<-e.received
				wg.Done()
			}
		}(engs[i])
	}

	unittest.AssertReturnsBefore(t, wg.Wait, 5*time.Second)
}

func (suite *MutableIdentityTableSuite) generateNodeIDRefreshers(nets []*network.Network) []*network.NodeIDRefresher {
	refreshers := make([]*network.NodeIDRefresher, len(nets))
	for i, net := range nets {
		refreshers[i] = network.NewNodeIDRefresher(suite.logger, suite.state, net.SetIDs)
	}
	return refreshers
}
