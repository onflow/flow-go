package test

import (
	"fmt"
	"testing"

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

//func (s *SubNetGeneratorTestSuite) TearDownTest() {
//	for _, net := range s.nets {
//		select {
//		// closes the network
//		case <-net.Done():
//			continue
//		case <-time.After(1 * time.Second):
//			s.Suite.Fail("could not stop the network")
//		}
//	}
//	fmt.Println("tests tear down")
//}

/*
Single Subnet tests
*/

// TestOneNodeOneSubNet evaluates CreateSubnets for creating a single subnet of one node
func (s *SubNetGeneratorTestSuite) TestOneNodeOneSubNet() {
	// single subnet of size 1 nodes
	s.SubNetSizeTestHelper(1, 1)
}

// TestTwoNodeOneSubNet evaluates CreateSubnets for creating a single subnet of two nodes
func (s *SubNetGeneratorTestSuite) TestTwoNodesOneSubNet() {
	// single subnet of size 2 nodes
	s.SubNetSizeTestHelper(2, 1)
}

// TestMultiNodeOneSubNet evaluates CreateSubnets for a single subnet of multiple nodes
func (s *SubNetGeneratorTestSuite) TestMultiNodesOneSubNet() {
	// single subnet of size 10 nodes
	s.SubNetSizeTestHelper(10, 1)
}

/*
Two Subnets tests
*/

// TestTwoNodesTwoSubNet evaluates CreateSubnets for creating two subnets of two nodes
func (s *SubNetGeneratorTestSuite) TestTwoNodesTwoSubNet() {
	// 2 subsets of size 2
	s.SubNetSizeTestHelper(2, 2)
}

// TestTenNodesTwoSubNet evaluates CreateSubnets for creating two subnets of 10 nodes
func (s *SubNetGeneratorTestSuite) TestTenNodesTwoSubNet() {
	// two subnet 10 nodes
	s.SubNetSizeTestHelper(10, 2)
}

/*
Multiple subnets
*/

// TestMultiNodeMultiSubNet evaluates CreateSubnets for dividing some nodes into same
// number of subnets, which should make all subnets receive at least one node
func (s *SubNetGeneratorTestSuite) TestMultiNodeMultiSubNet() {
	// divides 5 nodes into 5 subnets
	s.SubNetSizeTestHelper(5, 5)
}

// TestDoubleNodeMultiSubNet evaluates CreateSubnets for dividing double nodes into subnets
func (s *SubNetGeneratorTestSuite) TestDoubleNodeMultiSubNet() {
	// divides 10 nodes into 5 subnets
	s.SubNetSizeTestHelper(10, 5)
}

// SubNetSizeTestHelper creates subnets of different sizes
// and validates their correct correction
func (s *SubNetGeneratorTestSuite) SubNetSizeTestHelper(nodesNum, subsNum int) {
	subnets, err := CreateSubnets(nodesNum, subsNum)
	require.NoError(s.Suite.T(), err)

	for subnetID := range subnets {
		// all net instances should belong to either subnet zero or one
		if subnetID < 0 || subnetID >= nodesNum {
			require.Fail(s.Suite.T(), fmt.Sprintf("unidentified subnet id: %d", subnetID))
		}
		require.Equal(s.Suite.T(), len(subnets[subnetID]), nodesNum/subsNum)
	}
}
