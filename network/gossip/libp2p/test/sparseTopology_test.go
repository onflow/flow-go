package test

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/libp2p/message"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/middleware"
)

// SparseTopologyTestSuite test 1-k messaging in a sparsely connected network
// Topology is used to control how the node is divided into subsets
type SparseTopologyTestSuite struct {
	suite.Suite
	nets []*libp2p.Network    // used to keep track of the networks
	mws  []*libp2p.Middleware // used to keep track of the middlewares associated with networks
	ids  flow.IdentityList    // used to keep track of the identifiers associated with networks
}

// TestSparseTopologyTestSuite runs all tests in this test suit
func TestSparseTopologyTestSuite(t *testing.T) {
	suite.Run(t, new(SparseTopologyTestSuite))
}

// TestSparselyConnectedNetwork creates a network configuration with of 9 nodes with 3 subsets. The subsets are connected
// with each other with only one node
// 0,1,2,3 <-> 3,4,5,6 <-> 6,7,8,9
// Message sent by a node from one subset should be able to make it to nodes all subsets
func (stt *SparseTopologyTestSuite) TestSparselyConnectedNetwork() {
	stt.T().Skip()

	// total number of nodes in the network
	const count = 9
	// total number of subnets (should be less than count)
	const subsets = 3

	stt.ids = CreateIDs(count)

	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
	var err error
	stt.mws, err = CreateMiddleware(logger, stt.ids)
	require.NoError(stt.Suite.T(), err)

	tops := CreateSparseTopology(count, subsets)

	stt.nets, err = CreateNetworks(logger, stt.mws, stt.ids, 100, false, tops...)
	require.NoError(stt.Suite.T(), err)

	// create engines
	engs := make([]*MeshEngine, 0)
	for _, n := range stt.nets {
		eng := NewMeshEngine(stt.Suite.T(), n, count-1, engine.TestNetwork)
		engs = append(engs, eng)
	}

	// wait for nodes to heartbeat and discover each other
	time.Sleep(2 * time.Second)

	// node 0 broadcasting a message to all targets
	event := &message.Echo{
		Text: "hello from node 0",
	}
	require.NoError(stt.Suite.T(), engs[0].con.Submit(event, stt.ids.NodeIDs()...))

	// wait for message to be received by all recipients (excluding node 0)
	stt.checkMessageReception(engs, 1, count)

}

// TestDisjointedNetwork creates a network configuration of 9 nodes with 3 subsets. The subsets are not connected
// with each other.
// 0,1,2 <-> 3,4,5 <-> 6,7,8
// Message sent by a node from one subset should not be able to make to nodes in a different subset
// This test is created primarily to prove that topology is indeed honored by the networking layer since technically,
// each node does have the ip addresses of all other nodes and could just disregard topology all together and connect
// to every other node directly making the TestSparselyConnectedNetwork test meaningless
func (stt *SparseTopologyTestSuite) TestDisjointedNetwork() {
	stt.T().Skip()
	// total number of nodes in the network
	const count = 9
	// total number of subnets (should be less than count)
	const subsets = 3

	stt.ids = CreateIDs(count)

	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
	var err error
	stt.mws, err = CreateMiddleware(logger, stt.ids)
	require.NoError(stt.Suite.T(), err)

	tops := CreateDisjointedTopology(count, subsets)

	stt.nets, err = CreateNetworks(logger, stt.mws, stt.ids, 100, false, tops...)
	require.NoError(stt.Suite.T(), err)

	// create engines
	engs := make([]*MeshEngine, 0)
	for _, n := range stt.nets {
<<<<<<< HEAD
		eng := NewMeshEngine(stt.Suite.T(), n, count-1, engine.TestNetwork)
=======
		eng := NewMeshEngine(stt.Suite.T(), n, count-1, engine.PushTransactions)
>>>>>>> 5079ec686... made exchange engines single-entity
		engs = append(engs, eng)
	}

	// wait for nodes to heartbeat and discover each other
	// this is a sparse network so it may need a at least 3 seconds (1 for each subnet)
	time.Sleep(4 * time.Second)

	// node 0 broadcasting a message to ALL targets
	event := &message.Echo{
		Text: "hello from node 0",
	}
	require.NoError(stt.Suite.T(), engs[0].con.Submit(event, stt.ids.NodeIDs()...))

	// wait for message to be received by nodes only in subset 1 (excluding node 0)
	stt.checkMessageReception(engs, 1, subsets)
}

// TearDownTest closes the networks within a specified timeout
func (stt *SparseTopologyTestSuite) TearDownTest() {
	for _, net := range stt.nets {
		select {
		// closes the network
		case <-net.Done():
			continue
		case <-time.After(3 * time.Second):
			stt.Suite.Fail("could not stop the network")
		}
	}
	stt.ids = nil
	stt.mws = nil
	stt.nets = nil
}

// IndexBoundTopology is a topology implementation that limits the subset by indices of the identity list
type IndexBoundTopology struct {
	minIndex int
	maxIndex int
}

// Returns a subset of ids bounded by [minIndex, maxIndex) for the SparseTopology
func (ibt IndexBoundTopology) Subset(idList flow.IdentityList, _ int, _ string) (map[flow.Identifier]flow.Identity, error) {
	subsetLen := ibt.maxIndex - ibt.minIndex
	var result = make(map[flow.Identifier]flow.Identity, subsetLen)
	sub := idList[ibt.minIndex:ibt.maxIndex]
	for _, id := range sub {
		result[id.ID()] = *id
	}
	return result, nil
}

// CreateSparseTopology creates topologies for nodes such that subsets have one overlapping node
// e.g. top 1 - 0,1,2,3; top 2 - 3,4,5,6; top 3 - 6,7,8,9
func CreateSparseTopology(count int, subsets int) []middleware.Topology {
	tops := make([]middleware.Topology, count)
	subsetLen := count / subsets
	for i := 0; i < count; i++ {
		s := i / subsets          // which subset does this node belong to
		minIndex := s * subsetLen //minIndex is just a multiple subset length
		var maxIndex int
		if s == subsets-1 { // if this is the last subset
			maxIndex = count // then set max index to count since nodes so may not evenly divide by # of subsets
		} else {
			maxIndex = ((s + 1) * subsetLen) + 1 // a plus one to cause an overlap between subsets
		}
		st := IndexBoundTopology{
			minIndex: minIndex,
			maxIndex: maxIndex,
		}
		tops[i] = st
	}
	return tops
}

// CreateDisjointedTopology creates topologies for nodes such that subsets don't have any overlap
// e.g. top 1 - 0,1,2; top 2 - 3,4,5; top 3 - 6,7,8
func CreateDisjointedTopology(count int, subsets int) []middleware.Topology {
	tops := make([]middleware.Topology, count)
	subsetLen := count / subsets
	for i := 0; i < count; i++ {
		s := i / subsets          // which subset does this node belong to
		minIndex := s * subsetLen //minIndex is just a multiple subset length
		var maxIndex int
		if s == subsets-1 { // if this is the last subset
			maxIndex = count // then set max index to count since nodes so may not evenly divide by # of subsets
		} else {
			maxIndex = (s + 1) * subsetLen
		}
		st := IndexBoundTopology{
			minIndex: minIndex,
			maxIndex: maxIndex,
		}
		tops[i] = st
	}
	return tops
}

// checkMessageReception checks if engs[low:high) have received a message while all the other engs have not
func (stt SparseTopologyTestSuite) checkMessageReception(engs []*MeshEngine, low int, high int) {
	wg := sync.WaitGroup{}
	// fires a goroutine for all engines to listens for the incoming message
	for _, e := range engs[low:high] {
		wg.Add(1)
		go func(e *MeshEngine) {
			<-e.received
			wg.Done()
		}(e)
	}

	c := make(chan struct{})
	go func() {
		wg.Wait()
		c <- struct{}{}
	}()

	select {
	case <-c:
	case <-time.After(10 * time.Second):
		assert.Fail(stt.Suite.T(), "test timed out on broadcast dissemination")
	}

	// evaluates that all messages are received
	for i, e := range engs {
		if i >= low && i < high {
			assert.Len(stt.Suite.T(), e.event, 1, fmt.Sprintf("engine %d did not receive the message", i))
		} else {
			assert.Len(stt.Suite.T(), e.event, 0, fmt.Sprintf("engine %d received the message", i))
		}
	}
}
