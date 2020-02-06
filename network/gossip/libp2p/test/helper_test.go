package test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/network/gossip/libp2p"
)

type SubNetGeneratorTestSuite struct {
	suite.Suite
	nets map[int]*libp2p.Network
}

// TestSubNetGeneratorTestSuit runs all tests in this test suit
func TestSubNetGeneratorTestSuit(t *testing.T) {
	suite.Run(t, new(SubNetGeneratorTestSuite))
}

func (s *SubNetGeneratorTestSuite) SetupTest() {

}

func (s *SubNetGeneratorTestSuite) TearDownTest() {
	for _, net := range s.nets {
		select {
		// closes the network
		case <-net.Done():
			continue
		case <-time.After(1 * time.Second):
			s.Suite.Fail("could not stop the network")
		}
	}
	fmt.Println("tests tear down")
}

// TestOneNodeOneSubNet evaluates CreateSubnets for creating a single subnet of one node
func (s *SubNetGeneratorTestSuite) TestOneNodeOneSubNet() {
	// single subnet of size 1 nodes
	s.SingleSubNetHelper(1)
}

// TestTwoNodeOneSubNet evaluates CreateSubnets for creating a single subnet of two nodes
func (s *SubNetGeneratorTestSuite) TestTwoNodesOneSubNet() {
	// single subnet of size 2 nodes
	s.SingleSubNetHelper(2)
}

// TestMultiNodeOneSubNet evaluates CreateSubnets for creating a single subnet of multiple nodes
func (s *SubNetGeneratorTestSuite) TestMultiNodesOneSubNet() {
	// single subnet of size 10 nodes
	s.SingleSubNetHelper(10)
}

// SingleSubNetHelper creates single subnets of different sizes
func (s *SubNetGeneratorTestSuite) SingleSubNetHelper(nodesNum int) {
	subnets, err := CreateSubnets(nodesNum, 1)
	require.NoError(s.Suite.T(), err)
	require.Len(s.Suite.T(), subnets, nodesNum)
	for net := range subnets {
		// all net instances should belong to subnet zero
		require.Equal(s.Suite.T(), subnets[net], 0)
	}
}
