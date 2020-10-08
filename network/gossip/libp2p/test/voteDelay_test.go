package test

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	golog "github.com/ipfs/go-log"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/network/gossip/libp2p"
	"github.com/onflow/flow-go/utils/unittest"
)

type VoteDelayTestSuite struct {
	suite.Suite
	ConduitWrapper                      // used as a wrapper around conduit methods
	nets           []*libp2p.Network    // used to keep track of the networks
	mws            []*libp2p.Middleware // used to keep track of the middlewares associated with networks
	ids            flow.IdentityList    // used to keep track of the identifiers associated with networks
	engs           []*MeshEngine
}

func TestVoteDelayTestSuite(t *testing.T) {
	suite.Run(t, new(VoteDelayTestSuite))
}

func (m *VoteDelayTestSuite) SetupTest() {
	count := 10
	golog.SetAllLoggers(golog.LevelInfo)

	m.ids = CreateIDs(count)

	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
	mws, err := createMiddleware(logger, m.ids)
	require.NoError(m.Suite.T(), err)
	m.mws = mws

	nets, err := createNetworks(logger, m.mws, m.ids, 100, false)
	require.NoError(m.Suite.T(), err)
	m.nets = nets

	expectedMessageCount := 100

	// creating engines
	engs := make([]*MeshEngine, count)
	for i := range m.nets {
		engs[i] = NewMeshEngine(m.Suite.T(), m.nets[i], expectedMessageCount, engine.TestNetwork)
	}
	m.engs = engs

	// sleep for 2 seconds to let the nodes discover each other and form a mesh
	time.Sleep(2 * time.Second)
}

// TearDownTest closes the networks within a specified timeout
func (m *VoteDelayTestSuite) TearDownTest() {
	for _, net := range m.nets {
		select {
		// closes the network
		case <-net.Done():
			continue
		case <-time.After(3 * time.Second):
			m.Suite.Fail("could not stop the network")
		}
	}
}

func (m *VoteDelayTestSuite) TestBlockVoteSubmission() {
	// test that a each engine can send a message to the other using unicast (1-1)
	m.testMessageSendReceive(m.Unicast)

	// shutdown the last engine
	last := len(m.ids) - 1
	err := m.engs[last].con.Close()
	assert.NoError(m.T(), err)

	select {
	case <-m.nets[last].Done():
		break
	case <-time.After(3 * time.Second):
		m.Suite.Fail("could not stop the network")
	}

	// test that all remaining engines can still exchange messages
	m.engs = m.engs[:last]
	m.ids = m.ids[:last]
	m.nets = m.nets[:last]
	m.mws = m.mws[:last]

	m.testMessageSendReceive(m.Unicast)

}

func (m *VoteDelayTestSuite) testMessageSendReceive(send ConduitSendWrapperFunc) {

	count := len(m.engs)
	wg := sync.WaitGroup{}

	// each node broadcasting a message to all others
	for i := range m.nets {
		event := &message.TestMessage{
			Text: fmt.Sprintf("hello from node %v at %d", i, time.Now().UnixNano()),
		}

		// others keeps the identifier of all nodes except ith node
		others := m.ids.Filter(filter.Not(filter.HasNodeID(m.ids[i].NodeID))).NodeIDs()
		require.NoError(m.Suite.T(), send(event, m.engs[i].con, others...))
	}

	// fires a goroutine for each engine that listens to incoming messages
	for i := range m.nets {
		// wait group counts the number of messages received
		wg.Add(count - 1)
		go func(e *MeshEngine) {
			for x := 0; x < count-1; x++ {
				<-e.received
				fmt.Printf("Engine: %d recvd: %s", i, <-e.event)
				wg.Done()
			}
		}(m.engs[i])
	}

	unittest.AssertReturnsBefore(m.Suite.T(), wg.Wait, 1*time.Second)
}
