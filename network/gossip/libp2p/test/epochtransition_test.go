package test

import (
	"context"
	"fmt"
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

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/network/gossip/libp2p"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// total number of epochs to simulate
const epochs = 3

// total nodes
const nodeCount = 10

type EpochTransitionTestSuite struct {
	suite.Suite
	ConduitWrapper                      // used as a wrapper around conduit methods
	nets           []*libp2p.Network    // used to keep track of the networks
	mws            []*libp2p.Middleware // used to keep track of the middlewares associated with networks
	state          *protocol.ReadOnlyState
	snapshot       *protocol.Snapshot
	ids            flow.IdentityList
	currentEpoch   int // index of the current epoch
	epochIDs       []flow.IdentityList
	logger         zerolog.Logger
}

func TestEpochTransitionTestSuite(t *testing.T) {
	suite.Run(t, new(EpochTransitionTestSuite))
}

func (ts *EpochTransitionTestSuite) SetupTest() {
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
	ts.snapshot.On("Identities", mock2.Anything).Return(ts.ids, nil)

	// all nodes use the same state mock
	states := make([]*protocol.ReadOnlyState, nodeCount)
	for i := 0; i < nodeCount; i++ {
		states[i] = ts.state
	}

	// create networks using the mocked state and default topology
	nets, err := generateNetworks(ts.logger, ids, mws, 100, nil, states, false)
	require.NoError(ts.T(), err)
	ts.nets = nets

	ts.sendMessagesAndVerify(nil, nil, ts.Publish)
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

func (ts *EpochTransitionTestSuite) TestNewNodeAdded() {


	ids, mws, _, err := generateIDsMiddlewaresNetworks(1, ts.logger, 100, nil, []*protocol.ReadOnlyState{ts.state}, false)
	require.NoError(ts.T(), err)
	newID := ids[0]
	mw := mws[0]

	ts.ids = append(ts.ids, newID)
	for _, n := range ts.nets {
		n.EpochTransition(uint64(1), nil)
	}

	threshold := nodeCount / 2
	assert.Eventually(ts.T(), func() bool{
		connections := 0
		for _, id := range ts.ids {
			if id == newID {
				continue
			}
			connected, err := mw.IsConnected(id.NodeID)
			require.NoError(ts.T(),err)

			if connected {
				connections++
			}
		}
		// check if the new node has at least threshold connections
		if connections >= threshold {
			// the node has been inducted in the network ok
			return true
		}
		return false
	}, 5 * time.Second, time.Millisecond)
}

// sendMessagesAndVerify creates MeshEngines for each of the ids and then sends a message from each.
// It then verifies that all the engines at member indices received the message while those at nonmember indices
// didn't.
func (ts *EpochTransitionTestSuite) sendMessagesAndVerify(member []int, nonmember []int, send ConduitSendWrapperFunc) {

	// creating engines
	count := len(ts.nets)
	engs := make([]*MeshEngine, 0)
	wg := sync.WaitGroup{}

	// logs[i][j] keeps the message that node i sends to node j
	logs := make(map[int][]string)
	for i := range ts.nets {
		eng := NewMeshEngine(ts.Suite.T(), ts.nets[i], count-1, engine.TestNetwork)
		engs = append(engs, eng)
		logs[i] = make([]string, 0)
	}

	// allows nodes to find each other in case of Mulitcast and Publish
	optionalSleep(send)

	// each node broadcasting a message to all others
	for i := range ts.nets {
		event := &message.TestMessage{
			Text: fmt.Sprintf("hello from node %v", i),
		}

		// others keeps the identifier of all nodes except ith node
		others := ts.ids.Filter(filter.Not(filter.HasNodeID(ts.ids[i].NodeID))).NodeIDs()
		require.NoError(ts.Suite.T(), send(event, engs[i].con, others...))
	}

	ctx, cancel := context.WithCancel(context.Background())
	msgCnt := count - 1
	// fires a goroutine for each engine that listens to incoming messages
	for i := range ts.nets {
		wg.Add(msgCnt)
		go func(e *MeshEngine) {
			for x := 0; x < msgCnt; x++ {
				select {
				case <-ctx.Done():
					return
				case <-e.received:
					wg.Done()
				}
			}
		}(engs[i])
	}

	unittest.AssertReturnsBefore(ts.Suite.T(), wg.Wait, 30*time.Second)
	cancel()

	// evaluates that all messages are received
	for _, e := range engs {
		// confirms the number of received messages at each node
		assert.Len(ts.T(), e.event, msgCnt)
	}
}
