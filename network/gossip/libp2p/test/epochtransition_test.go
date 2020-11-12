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
	"github.com/onflow/flow-go/network/gossip/libp2p"
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
	nets         []*libp2p.Network
	mws          []*libp2p.Middleware
	idRefreshers []*libp2p.NodeIDRefresher
	engines      []*MeshEngine
	state        *protocol.ReadOnlyState
	snapshot     *protocol.Snapshot
	ids          flow.IdentityList
	logger       zerolog.Logger
}

func TestEpochTransitionTestSuite(t *testing.T) {
	suite.Run(t, new(MutableIdentityTableSuite))
}

func (ts *MutableIdentityTableSuite) SetupTest() {
	rand.Seed(time.Now().UnixNano())
	nodeCount := 10
	ts.logger = zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
	golog.SetAllLoggers(golog.LevelError)

	// create ids
	ids, mws := generateIDsAndMiddlewares(ts.T(), nodeCount, ts.logger)
	ts.ids = ids
	ts.mws = mws

	// setup state related mocks
	final := unittest.BlockHeaderFixture()
	ts.state = new(protocol.ReadOnlyState)
	ts.snapshot = new(protocol.Snapshot)
	ts.snapshot.On("Head").Return(&final, nil)
	ts.snapshot.On("Phase").Return(flow.EpochPhaseCommitted, nil)
	ts.snapshot.On("Identities", testifymock.Anything).Return(
		func(flow.IdentityFilter) flow.IdentityList { return ts.ids },
		func(flow.IdentityFilter) error { return nil })
	ts.state.On("Final").Return(ts.snapshot, nil)

	// create networks using the mocked state and default topology
	nets := generateNetworks(ts.T(), ts.logger, ids, mws, 100, nil, false)
	ts.nets = nets

	// generate the refreshers
	ts.idRefreshers = ts.generateNodeIDRefreshers(nets)

	// generate the engines
	ts.engines = generateEngines(ts.T(), nets)
}

// TearDownTest closes the networks within a specified timeout
func (ts *MutableIdentityTableSuite) TearDownTest() {
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

// TestNewNodeAdded tests that when a new node is added to the identity list
// (ie. as a result of a EpochSetup event) that it can connect to the network.
func (ts *MutableIdentityTableSuite) TestNewNodeAdded() {

	// create the id, middleware and network for a new node
	ids, mws, nets := generateIDsMiddlewaresNetworks(ts.T(), 1, ts.logger, 100, nil, false)
	newID := ids[0]
	newMiddleware := mws[0]

	newIDs := append(ts.ids, ids...)
	ts.ids = newIDs

	// create a new refresher
	newIDRefresher := ts.generateNodeIDRefreshers(nets)
	newIDRefreshers := append(ts.idRefreshers, newIDRefresher...)

	// create the engine for the new node
	newEngine := generateEngines(ts.T(), nets)
	newEngines := append(ts.engines, newEngine...)

	// trigger the identity table change event
	for _, n := range newIDRefreshers {
		n.OnIdentityTableChanged()
	}

	// check if the new node has sufficient connections with the existing nodes
	// if it does, then it has been inducted successfully in the network
	checkConnectivity(ts.T(), newMiddleware, newIDs.Filter(filter.Not(filter.HasNodeID(newID.NodeID))))

	// check that all the engines on this new epoch can talk to each other
	sendMessagesAndVerify(ts.T(), newIDs, newEngines, ts.Publish)
}

// TestNodeRemoved tests that when an existing node is removed from the identity
// list (ie. as a result of an ejection or transition into an epoch where that node
// has un-staked) that it cannot connect to the network.
func (ts *MutableIdentityTableSuite) TestNodeRemoved() {

	// choose a random node to remove
	removeIndex := rand.Intn(len(ts.ids))
	removedID := ts.ids[removeIndex]

	// remove the identity at that index from the ids
	newIDs := ts.ids.Filter(filter.Not(filter.HasNodeID(removedID.NodeID)))
	ts.ids = newIDs

	// create a list of engines except for the removed node
	var newEngines []*MeshEngine
	for i, eng := range ts.engines {
		if i == removeIndex {
			continue
		}
		newEngines = append(newEngines, eng)
	}

	// trigger an epoch phase change for all nodes
	// from flow.EpochPhaseStaking to flow.EpochPhaseSetup
	for _, n := range ts.idRefreshers {
		n.OnIdentityTableChanged()
	}

	// check that all remaining engines can still talk to each other
	sendMessagesAndVerify(ts.T(), newIDs, newEngines, ts.Publish)

	// TODO check that messages to/from evicted node are not delivered
}

// checkConnectivity checks that the middleware of a node is directly connected
// to at least half of the other nodes.
func checkConnectivity(t *testing.T, mw *libp2p.Middleware, ids flow.IdentityList) {
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

func (ts *MutableIdentityTableSuite) generateNodeIDRefreshers(nets []*libp2p.Network) []*libp2p.NodeIDRefresher {
	refreshers := make([]*libp2p.NodeIDRefresher, len(nets))
	for i, net := range nets {
		refreshers[i] = libp2p.NewNodeIDRefresher(ts.logger, ts.state, net.SetIDs)
	}
	return refreshers
}
