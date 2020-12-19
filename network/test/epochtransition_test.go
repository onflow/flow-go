package test

import (
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/go-log"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/network/p2p"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
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
	nets         []*p2p.Network
	mws          []*p2p.Middleware
	idRefreshers []*p2p.NodeIDRefresher
	engines      []*MeshEngine
	state        *mockprotocol.State
	snapshot     *mockprotocol.Snapshot
	ids          flow.IdentityList
	logger       zerolog.Logger
}

func TestEpochTransitionTestSuite(t *testing.T) {
	suite.Run(t, new(MutableIdentityTableSuite))
}

func (suite *MutableIdentityTableSuite) SetupTest() {
	rand.Seed(time.Now().UnixNano())
	nodeCount := 10
	suite.logger = zerolog.New(os.Stderr).Level(zerolog.InfoLevel)
	log.SetAllLoggers(log.LevelError)

	// create ids
	ids, mws := GenerateIDsAndMiddlewares(suite.T(), nodeCount, !DryRun, suite.logger)
	suite.ids = ids
	suite.mws = mws

	// setup state related mocks
	final := unittest.BlockHeaderFixture()
	suite.state = new(mockprotocol.State)
	suite.snapshot = new(mockprotocol.Snapshot)
	suite.snapshot.On("Head").Return(&final, nil)
	suite.snapshot.On("Phase").Return(flow.EpochPhaseCommitted, nil)
	suite.snapshot.On("Identities", mock.Anything).Return(
		func(flow.IdentityFilter) flow.IdentityList { return suite.ids },
		func(flow.IdentityFilter) error { return nil })
	suite.state.On("Final").Return(suite.snapshot, nil)

	// all nodes use the same state mock
	states := make([]*mockprotocol.State, nodeCount)
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

// TestNewNodeAdded tests that when a new node is added to the identity list e.g. on an epoch,
// then it can connect to the network.
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

	// update IDs for all the networks (simulating an epoch)
	for _, n := range newIDRefreshers {
		n.OnIdentityTableChanged()
	}

	// wait for all the connections and disconnections to occur at the network layer
	time.Sleep(2 * time.Second)

	// check if the new node has sufficient connections with the existing nodes
	// if it does, then it has been inducted successfully in the network
	checkConnectivity(suite.T(), newMiddleware, newIDs.Filter(filter.Not(filter.HasNodeID(newID.NodeID))))

	// check that all the engines on this new epoch can talk to each other using any of the three networking primitives
	suite.exchangeMessages(newIDs, newEngines, nil, nil, suite.Publish)
	suite.exchangeMessages(newIDs, newEngines, nil, nil, suite.Multicast)
	suite.exchangeMessages(newIDs, newEngines, nil, nil, suite.Unicast)
}

// TestNodeRemoved tests that when an existing node is removed from the identity
// list (ie. as a result of an ejection or transition into an epoch where that node
// has un-staked) then it cannot connect to the network.
func (suite *MutableIdentityTableSuite) TestNodeRemoved() {

	// choose a random node to remove
	removeIndex := rand.Intn(len(suite.ids) - 1)
	removedID := suite.ids[removeIndex]
	removedEngines := suite.engines[removeIndex : removeIndex+1]

	fmt.Printf("Removed ID: %s at index %d\n", removedID.String(), removeIndex)

	// remove the identity at that index from the ids
	newIDs := suite.ids.Filter(filter.Not(filter.HasNodeID(removedID.NodeID)))
	removedIDs := suite.ids[removeIndex : removeIndex+1]
	suite.ids = newIDs

	// create a list of engines except for the removed node
	var newEngines []*MeshEngine
	for i, eng := range suite.engines {
		if i == removeIndex {
			continue
		}
		newEngines = append(newEngines, eng)
	}

	// update IDs for all the networks except the one that was removed
	// (since the evicted node may just continue with the old list)
	for _, n := range suite.idRefreshers {
		//if i == removeIndex {
		//	continue
		//}
		n.OnIdentityTableChanged()
	}

	// wait for all the connections and disconnections to occur at the network layer
	time.Sleep(2 * time.Second)

	// check that all remaining engines can still talk to each other while the ones removed can't
	// using any of the three networking primitives
	suite.exchangeMessages(newIDs, newEngines, removedIDs, removedEngines, suite.Publish)
	suite.exchangeMessages(newIDs, newEngines, removedIDs, removedEngines, suite.Multicast)
	suite.exchangeMessages(newIDs, newEngines, removedIDs, removedEngines, suite.Unicast)
}

// checkConnectivity checks that the middleware of a node is directly connected
// to at least half of the other nodes.
func checkConnectivity(t *testing.T, mw *p2p.Middleware, ids flow.IdentityList) {
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

// exchangeMessages verifies that allowed engines can successfully exchange messages while disallowed engines can't
func (suite *MutableIdentityTableSuite) exchangeMessages(
	allowedIDs flow.IdentityList,
	allowedEngs []*MeshEngine,
	disallowedIDs flow.IdentityList,
	disallowedEngs []*MeshEngine,
	send ConduitSendWrapperFunc) {

	count := len(allowedEngs)
	expectedMsgCnt := count - 1

	// send a message from each of the allowed engine to the other allowed engines
	for i, allowedEng := range allowedEngs {

		fromID := allowedIDs[i].NodeID
		targetIDs := allowedIDs.Filter(filter.Not(filter.HasNodeID(allowedIDs[i].NodeID)))

		suite.sendMessage(fromID, allowedEng, targetIDs, send)
	}

	// send a message from each of the allowed engine to each of the disallowed engines
	//for i, allowedEng := range allowedEngs {
	//
	//	fromID := allowedIDs[i].NodeID
	//	targetIDs := disallowedIDs
	//
	//	suite.sendMessage(fromID, allowedEng, targetIDs, send)
	//}

	// send a message from each of the disallowed engine to each of the allowed engines
	//for i, disallowedEng := range disallowedEngs {
	//
	//	fromID := disallowedIDs[i].NodeID
	//	targetIDs := allowedIDs
	//
	//	suite.sendMessage(fromID, disallowedEng, targetIDs, send)
	//
	//	fmt.Printf(">>>>>>>>>>>>>>>>> \n disallowed: %s\n", disallowedIDs[i].String())
	//}

	wg := sync.WaitGroup{}
	// fires a goroutine for each of the allowed engine to listen for incoming messages
	for i := range allowedEngs {
		wg.Add(expectedMsgCnt)
		go func(e *MeshEngine) {
			for x := 0; x < expectedMsgCnt; x++ {
				<-e.received
				wg.Done()
			}
		}(allowedEngs[i])
	}

	// assert that all allowed engines received expectedMsgCnt number of messages
	unittest.AssertReturnsBefore(suite.T(), wg.Wait, 5*time.Second)

	// assert that the disallowed engines didn't receive any message
	for _, eng := range disallowedEngs {
		unittest.RequireNeverReturnBefore(suite.T(), func() {
			<-eng.received
		}, time.Millisecond, fmt.Sprintf("%s engine should not have recevied message", eng.originID.String()))
	}
}

func (suite *MutableIdentityTableSuite) sendMessage(
	fromID flow.Identifier,
	fromEngine *MeshEngine,
	toIDs flow.IdentityList,
	send ConduitSendWrapperFunc) {

	primitive := runtime.FuncForPC(reflect.ValueOf(send).Pointer()).Name()
	requireError := strings.Contains(primitive, "Unicast")

	event := &message.TestMessage{
		Text: fmt.Sprintf("hello from node %s using %s", fromID.String(), primitive),
	}

	err := send(event, fromEngine.con, toIDs.NodeIDs()...)
	if requireError {
		require.Error(suite.T(), err)
		return
	}
	require.NoError(suite.T(), err)
}

func (suite *MutableIdentityTableSuite) generateNodeIDRefreshers(nets []*p2p.Network) []*p2p.NodeIDRefresher {
	refreshers := make([]*p2p.NodeIDRefresher, len(nets))
	for i, net := range nets {
		refreshers[i] = p2p.NewNodeIDRefresher(suite.logger, suite.state, net.SetIDs)
	}
	return refreshers
}
