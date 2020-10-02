package test

import (
	"os"
	"testing"
	"time"

	golog "github.com/ipfs/go-log"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	mock2 "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/gossip/libp2p"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
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


	ids, _, _, err := generateIDsMiddlewaresNetworks(1, ts.logger, 100, nil, []*protocol.ReadOnlyState{ts.state}, false)
	require.NoError(ts.T(), err)
	newID := ids[0]


	ts.ids = append(ts.ids, newID)
	for _, n := range ts.nets {
		n.EpochTransition(uint64(1), nil)
	}

	time.Sleep(5 * time.Second)
}

// sendMessageAndVerify creates MeshEngine for each of the ids and then sends a message from each.
// It then verifies that all the engines at member indices received the message while those at nonmember indices
// didn't.
//func (ts *EpochTransitionTestSuite) sendMessageAndVerify(member []int, nonmember []int, send ConduitSendWrapperFunc) {
//
//	// creating engines
//	count := len(ts.nets)
//	engs := make([]*MeshEngine, 0)
//	wg := sync.WaitGroup{}
//
//	// logs[i][j] keeps the message that node i sends to node j
//	logs := make(map[int][]string)
//	for i := range ts.nets {
//		eng := NewMeshEngine(ts.Suite.T(), ts.nets[i], count-1, engine.TestNetwork)
//		engs = append(engs, eng)
//		logs[i] = make([]string, 0)
//	}
//
//	// allows nodes to find each other in case of Mulitcast and Publish
//	optionalSleep(send)
//
//	// Each node broadcasting a message to all others
//	for i := range ts.nets {
//		event := &message.TestMessage{
//			Text: fmt.Sprintf("hello from node %v", i),
//		}
//
//		// others keeps the identifier of all nodes except ith node
//		others := ts.ids.Filter(filter.Not(filter.HasNodeID(ts.ids[i].NodeID))).NodeIDs()
//		require.NoError(ts.Suite.T(), send(event, engs[i].con, others...))
//	}
//
//	ctx, cancel := context.WithCancel(context.Background())
//	// fires a goroutine for each engine that listens to incoming messages
//	for i := range ts.nets {
//		wg.Add(1)
//		go func(e *MeshEngine) {
//			for x := 0; x < count-1; x++ {
//				select {
//				<-ctx.done:
//					return
//					<-e.received:
//
//				<-e.received
//			}
//			wg.Done()
//		}(engs[i])
//	}
//
//	unittest.AssertReturnsBefore(ts.Suite.T(), wg.Wait, 30*time.Second)
//
//	// evaluates that all messages are received
//	for index, e := range engs {
//		// confirms the number of received messages at each node
//		if len(e.event) != (count - 1) {
//			assert.Fail(ts.Suite.T(),
//				fmt.Sprintf("Message reception mismatch at node %v. Expected: %v, Got: %v", index, count-1, len(e.event)))
//		}
//
//		// extracts failed messages
//		receivedIndices, err := extractSenderID(count, e.event, "hello from node")
//		require.NoError(ts.Suite.T(), err)
//
//		for j := 0; j < count; j++ {
//			// evaluates self-gossip
//			if j == index {
//				assert.False(ts.Suite.T(), (receivedIndices)[index], fmt.Sprintf("self gossiped for node %v detected", index))
//			}
//			// evaluates content
//			if !(receivedIndices)[j] {
//				assert.False(ts.Suite.T(), (receivedIndices)[index],
//					fmt.Sprintf("Message not found in node #%v's messages. Expected: Message from node %v. Got: No message", index, j))
//			}
//		}
//	}
//}
