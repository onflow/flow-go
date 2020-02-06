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

func (s *SubNetGeneratorTestSuite) TestTearDown() {
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

// TestOneNodeOneSubNet evaluates CreateSubnets for creating a single subnet
func (s *SubNetGeneratorTestSuite) TestOneNodeOneSubNet() {
	subnets, err := CreateSubnets(1, 1)
	require.NoError(s.Suite.T(), err)
	require.Len(s.Suite.T(), subnets, 1)
}

// TestOneNodeOneSubNet evaluates CreateSubnets for creating a single subnet
func (s *SubNetGeneratorTestSuite) TestTwoNodesOneSubNet() {
	subnets, err := CreateSubnets(2, 1)
	require.NoError(s.Suite.T(), err)
	require.Len(s.Suite.T(), subnets, 2)
}
