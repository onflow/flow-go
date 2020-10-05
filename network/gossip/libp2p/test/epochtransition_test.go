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
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	mock2 "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/network/gossip/libp2p"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type EpochTransitionTestSuite struct {
	suite.Suite
	ConduitWrapper
	nets         []*libp2p.Network
	mws          []*libp2p.Middleware
	engines      []*MeshEngine
	state        *protocol.ReadOnlyState
	snapshot     *protocol.Snapshot
	ids          flow.IdentityList
	currentEpoch int //counter to track the current epoch
	logger       zerolog.Logger
}

func TestEpochTransitionTestSuite(t *testing.T) {
	suite.Run(t, new(EpochTransitionTestSuite))
}

func (ts *EpochTransitionTestSuite) SetupTest() {
	nodeCount := 10
	golog.SetAllLoggers(golog.LevelDebug)
	ts.logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()

	// create ids
	ids, mws, err := generateIDsAndMiddlewares(nodeCount, ts.logger)
	require.NoError(ts.T(), err)
	ts.ids = ids
	ts.mws = mws

	// setup state related mocks
	ts.state = new(protocol.ReadOnlyState)
	ts.snapshot = new(protocol.Snapshot)
	ts.state.On("Final").Return(ts.snapshot, nil)

	ts.currentEpoch = 0
	ts.updateSnapshot(0, ts.ids)

	// all nodes use the same state mock
	states := make([]*protocol.ReadOnlyState, nodeCount)
	for i := 0; i < nodeCount; i++ {
		states[i] = ts.state
	}

	// create networks using the mocked state and default topology
	nets, err := generateNetworks(ts.logger, ids, mws, 100, nil, states, false)
	require.NoError(ts.T(), err)
	ts.nets = nets

	// generate the engines
	ts.engines = generateEngines(ts.T(), nets)

	// ensure that nodes can communicate with each other
	sendMessagesAndVerify(ts.T(), ts.ids, ts.engines, ts.Publish)
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

// TestNewNodeAdded tests that an additional node in a new epoch get connected to other nodes and can exchange messages
func (ts *EpochTransitionTestSuite) TestNewNodeAdded() {

	// create the id, middleware and network for a new node
	ids, mws, nets, err := generateIDsMiddlewaresNetworks(1, ts.logger, 100, nil, []*protocol.ReadOnlyState{ts.state}, false)
	require.NoError(ts.T(), err)
	newMiddleware := mws[0]

	newIDs := append(ts.ids, ids...)
	newNetworks := append(ts.nets, nets...)

	// increment epoch
	ts.currentEpoch = ts.currentEpoch + 1

	// update snapshot mock to return the new ids
	ts.updateSnapshot(ts.currentEpoch, newIDs)

	// create the engine for the new node
	newEngine := generateEngines(ts.T(), nets)
	newEngines := append(ts.engines, newEngine...)

	// trigger an epoch transition for all networks
	for _, n := range newNetworks {
		n.EpochTransition(uint64(ts.currentEpoch), nil)
	}

	threshold := len(ts.ids) / 2

	// check if the new node has at least threshold connections with the existing nodes
	// if it does, then it has been inducted successfully in the network
	assert.Eventually(ts.T(), func() bool {
		connections := 0
		for _, id := range ts.ids {
			connected, err := newMiddleware.IsConnected(*id)
			require.NoError(ts.T(), err)
			if connected {
				connections++
			}
		}
		return connections >= threshold
	}, 5*time.Second, time.Millisecond)

	// check that all the engines on this new epoch can talk to each other
	sendMessagesAndVerify(ts.T(), newIDs, newEngines, ts.Publish)
}

// TestNodeRemoved tests that a node that is removed in a new epoch gets disconnected from other nodes
func (ts *EpochTransitionTestSuite) TestNodeRemoved() {
	// choose a random index
	removeIndex := rand.Intn(len(ts.ids))
	fmt.Printf("\nREmoving %s\n", ts.ids[removeIndex].Address)

	// remove the identity at that index from the ids
	newIDs := ts.ids.Filter(filter.Not(filter.HasNodeID(ts.ids[removeIndex].NodeID)))

	// increment epoch
	ts.currentEpoch = ts.currentEpoch + 1

	// update snapshot mock to return the new ids
	ts.updateSnapshot(ts.currentEpoch, newIDs)

	// trigger an epoch transition for all nodes except the evicted one
	for i, n := range ts.nets {
		if i == removeIndex {
			continue
		}
		n.EpochTransition(uint64(ts.currentEpoch), nil)
	}

	removedID := ts.ids[removeIndex]
	// check that the evicted node has no connections
	assert.Eventually(ts.T(), func() bool {
		for i, id := range newIDs {
			connected, err := ts.mws[i].IsConnected(*removedID)
			require.NoError(ts.T(), err)
			if connected {
				fmt.Printf("\n\n >>>>>>> still connected with %s\n\n", id.Address)
				return false
			}
		}
		return true
	}, 30*time.Second, 10*time.Millisecond)
}

// sendMessagesAndVerify sends a message from each engine to the other engines and verifies that all the messages are
// delivered
func sendMessagesAndVerify(t *testing.T, ids flow.IdentityList, engs []*MeshEngine, send ConduitSendWrapperFunc) {

	// allows nodes to find each other in case of Mulitcast and Publish
	optionalSleep(send)

	count := len(engs)
	expectedMsgCnt := count - 1

	// each node broadcasting a message to all others
	for i, eng := range engs {
		event := &message.TestMessage{
			Text: fmt.Sprintf("hello from node %d to %d other nodes", i, count),
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

// updateSnapshot sets up the snapshot mock to return ids corresponding to epochs
func (ts *EpochTransitionTestSuite) updateSnapshot(epoch int, ids flow.IdentityList) {
	ts.snapshot.On("Identities",
		mock2.MatchedBy(func(_ flow.IdentityFilter) bool { return ts.currentEpoch == epoch })).Return(ids, nil)
}
