package test

import (
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/middleware"
)

// SparseTopologyTest test 1-k messaging in a sparsely connected network
type SparseTopologyTest struct {
	suite.Suite
	nets []*libp2p.Network    // used to keep track of the networks
	mws  []*libp2p.Middleware // used to keep track of the middlewares associated with networks
	ids  flow.IdentityList    // used to keep track of the identifiers associated with networks
}

// TestSparseTopologyTestSuite runs all tests in this test suit
func TestSparseTopologyTestSuite(t *testing.T) {
	suite.Run(t, new(MeshNetTestSuite))
}

func (m *SparseTopologyTest) TestSparselyConnectedNetwork() {

	// defines total number of nodes in our network
	const count = 9
	const subsets = 3

	ids := CreateIDs(count)

	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
	mws, err := CreateMiddleware(logger, m.ids)
	require.NoError(m.Suite.T(), err)

	tops := CreateSparseTopology(count, subsets)

	nets, err := CreateNetworks(logger, mws, ids, 100, false, tops...)
	require.NoError(m.Suite.T(), err)

}

func (m *SparseTopologyTest) TestDisjointedNetwork() {

}

// TearDownTest closes the networks within a specified timeout
func (s *SparseTopologyTest) TearDownTest() {
	for _, net := range s.nets {
		select {
		// closes the network
		case <-net.Done():
			continue
		case <-time.After(3 * time.Second):
			s.Suite.Fail("could not stop the network")
		}
	}
}

// IndexBoundTopology a topology implementation that limits the subset by indices of the identity list
type IndexBoundTopology struct {
	minIndex int
	maxIndex int
}

// Returns a subset of ids bounded by [minIndex, maxIndex) for the SparseTopology
func (st IndexBoundTopology) Subset(idList flow.IdentityList, _ int, _ string) (map[flow.Identifier]flow.Identity, error) {
	subsetLen := st.maxIndex - st.minIndex
	var result = make(map[flow.Identifier]flow.Identity, subsetLen)
	sub := idList[st.minIndex:st.maxIndex]
	for _, id := range sub {
		result[id.ID()] = *id
	}
	return result, nil
}

func CreateSparseTopology(count int, subsets int) []middleware.Topology {
	tops := make([]middleware.Topology, count)
	subsetLen := count / subsets
	for i := 0; i < count; i++ {
		s := i % subsets
		minIndex := s * subsetLen
		var maxIndex int
		if i == count-1 {
			maxIndex = count // subsets may not evenly divide total nodes, hence last subset may end up having more nodes
		} else {
			maxIndex = ((s + 1) * subsetLen) + 1 // a plus one to cause an overlap
		}
		st := IndexBoundTopology{
			minIndex: minIndex,
			maxIndex: maxIndex,
		}
		tops[i] = st
	}
	return tops
}
