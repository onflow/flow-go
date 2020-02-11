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
	s.SubNetSizeTestHelper(1, 1, 0)
}

// TestTwoNodeOneSubNet evaluates CreateSubnets for creating a single subnet of two nodes
func (s *SubNetGeneratorTestSuite) TestTwoNodesOneSubNet() {
	// single subnet of size 2 nodes
	s.SubNetSizeTestHelper(2, 1, 0)
}

// TestMultiNodeOneSubNet evaluates CreateSubnets for a single subnet of multiple nodes
func (s *SubNetGeneratorTestSuite) TestMultiNodesOneSubNet() {
	// single subnet of size 10 nodes
	s.SubNetSizeTestHelper(10, 1, 0)
}

/*
Two Subnets tests
*/

// TestTwoNodesTwoSubNet evaluates CreateSubnets for creating two subnets of two nodes
func (s *SubNetGeneratorTestSuite) TestTwoNodesTwoSubNet() {
	// 2 subsets of size 2
	s.SubNetSizeTestHelper(2, 2, 0)
}

// TestTenNodesTwoSubNet evaluates CreateSubnets for creating two subnets of 10 nodes
func (s *SubNetGeneratorTestSuite) TestTenNodesTwoSubNet() {
	// two subnet 10 nodes
	s.SubNetSizeTestHelper(10, 2, 0)
}

/*
Multiple subnets
*/

// TestMultiNodeMultiSubNet evaluates CreateSubnets for dividing some nodes into same
// number of subnets, which should make all subnets receive at least one node
func (s *SubNetGeneratorTestSuite) TestMultiNodeMultiSubNet() {
	// divides 5 nodes into 5 subnets
	s.SubNetSizeTestHelper(5, 5, 0)
}

// TestDoubleNodeMultiSubNet evaluates CreateSubnets for dividing double nodes into subnets
func (s *SubNetGeneratorTestSuite) TestDoubleNodeMultiSubNet() {
	// divides 10 nodes into 5 subnets
	s.SubNetSizeTestHelper(10, 5, 0)
}

/*
Liked subnets
*/
// TestTenNodesTwoSubNet evaluates CreateSubnets for creating two subnets of 10 nodes linked with one node from
// each one
func (s *SubNetGeneratorTestSuite) TestTenNodesTwoLinkedSubNet() {
	// two subnet 10 nodes
	s.SubNetSizeTestHelper(10, 2, 1)
}

// SubNetSizeTestHelper creates subnets of different sizes
// and validates their correct correction
func (s *SubNetGeneratorTestSuite) SubNetSizeTestHelper(nodeNum, subNum, linkNum int) {
	subnets, idMap, err := CreateSubnets(nodeNum, subNum, linkNum)
	require.NoError(s.Suite.T(), err)

	// iterates on the subnet ids (not nodes) in the ids map
	for subnetID := range subnets {
		// all subnet ids should lay in range [0, subnetID-1]
		if subnetID < 0 || subnetID >= nodeNum {
			require.Fail(s.Suite.T(), fmt.Sprintf("unidentified subnet id: %d", subnetID))
		}
		// size of each subnet should be equal to nodeNum/subNum + external links between subnets
		require.Equal(s.Suite.T(), nodeNum/subNum+(linkNum), len(idMap[subnetID]))
	}

	// iterates on the subnet ids (not nodes) in the nets map
	for subnetID := range idMap {
		// all subnet ids should lay in range [0, subnetID-1]
		if subnetID < 0 || subnetID >= nodeNum {
			require.Fail(s.Suite.T(), fmt.Sprintf("unidentified subnet id: %d", subnetID))
		}
		// size of each subnet should be equal to nodeNum/subNum + external links between subnets
		require.Equal(s.Suite.T(), (nodeNum/subNum)+(linkNum), len(idMap[subnetID]))
	}
}
