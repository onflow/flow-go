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

/*
Single Subnet tests
*/

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

// TestMultiNodeOneSubNet evaluates CreateSubnets for a single subnet of multiple nodes
func (s *SubNetGeneratorTestSuite) TestMultiNodesOneSubNet() {
	// single subnet of size 10 nodes
	s.SingleSubNetHelper(10)
}

// TestTwoNodeTwoSubnet evaluates CreateSubnets for dividing two nodes into two subnets
// of size one
func (s *SubNetGeneratorTestSuite) TestTwoNodeTwoSubNet() {
	// two subnet of size 10 nodes
	s.SingleSubNetHelper(10)
}

// SingleSubNetHelper creates single subnets of different sizes
// nodesNum is the total number of nodes
func (s *SubNetGeneratorTestSuite) SingleSubNetHelper(nodesNum int) {
	subnets, err := CreateSubnets(nodesNum, 1)
	require.NoError(s.Suite.T(), err)
	require.Len(s.Suite.T(), subnets, nodesNum)
	for net := range subnets {
		// all net instances should belong to subnet zero
		require.Equal(s.Suite.T(), subnets[net], 0)
	}
}

/*
Two Subnets tests
*/

// TestOneNodeTwoSubnet evaluates CreateSubnets for creating dividing one node into two subnets
// one of size zero and the other one of size 1
func (s *SubNetGeneratorTestSuite) TestOneNodeTwoSubNet() {
	// two subnet of size 1 nodes
	s.TwoSubNetHelper(1)
}

// TestOddNodesTwoSubNet evaluates CreateSubnets for creating two subnets of odd number of nodes
// the size difference of the subsets should be one
func (s *SubNetGeneratorTestSuite) TestOddNodesTwoSubNet() {
	// two subnet of size 9 nodes
	s.TwoSubNetHelper(9)
}

// TestEvenNodesTwoSubNet evaluates CreateSubnets for creating two subnets of even number of nodes
func (s *SubNetGeneratorTestSuite) TestEvenNodesTwoSubNet() {
	// two subnet of size 10 nodes
	s.TwoSubNetHelper(10)
}

// TwoSubNetHelper creates two subnets of different sizes
// nodesNum is the total number of nodes
func (s *SubNetGeneratorTestSuite) TwoSubNetHelper(nodesNum int) {
	subnets, err := CreateSubnets(nodesNum, 2)
	require.NoError(s.Suite.T(), err)
	require.Len(s.Suite.T(), subnets, nodesNum)

	// keeps size of subnets
	sub1 := 0
	sub2 := 0

	for net := range subnets {
		// all net instances should belong to either subnet zero or one
		if subnets[net] == 0 {
			sub1++
		} else if subnets[net] == 1 {
			sub2++
		} else {
			require.Fail(s.Suite.T(), fmt.Sprintf("unidentified subnet id: %d", subnets[net]))
		}
	}

	// evaluates size of each subnet
	if nodesNum < 2 {
		require.Equal(s.Suite.T(), sub1, nodesNum)
	} else {
		require.Equal(s.Suite.T(), sub1, nodesNum/2)
		// to cover odd number of nodes
		require.Equal(s.Suite.T(), sub2, nodesNum-nodesNum/2)
	}
}

/*
Multiple subnets
*/

// TestMultiNodeMultiSubNet evaluates CreateSubnets for dividing some nodes into same
// number of subnets, which should make all subnets receive at least one node
func (s *SubNetGeneratorTestSuite) TestMultiNodeMultiSubNet() {
	// divides 5 nodes into 5 subnets
	s.MultiSubNetHelper(5, 5)
}

// TestDoubleNodeMultiSubNet evaluates CreateSubnets for dividing double nodes into subnets
func (s *SubNetGeneratorTestSuite) TestDoubleNodeMultiSubNet() {
	// divides 10 nodes into 5 subnets
	s.MultiSubNetHelper(10, 5)
}

// MultiSubNetHelper creates multiple subnets of different sizes
// nodesNum is the total number of nodes
// subnetNum is the total number of subnets
func (s *SubNetGeneratorTestSuite) MultiSubNetHelper(nodesNum int, subnetNum int) {
	subnets, err := CreateSubnets(nodesNum, subnetNum)
	require.NoError(s.Suite.T(), err)
	require.Len(s.Suite.T(), subnets, nodesNum)

	// keeps size of subnets
	subs := make([]int, subnetNum)

	for net, subid := range subnets {
		// all net should have a subnet id between 0 to subnetNum - 1
		if subid < 0 || subid >= subnetNum {
			require.Fail(s.Suite.T(), fmt.Sprintf("unidentified subnet id: %d", subnets[net]))
		} else {
			// keeps record of size of each subnets
			subs[subid]++
		}
	}

	// evaluates size of each subnet
	for i := range subs {
		require.Equal(s.Suite.T(), subs[i], nodesNum/subnetNum)
	}
}
