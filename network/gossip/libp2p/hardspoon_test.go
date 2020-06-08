package libp2p

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// HardSpooningTestSuite tests that the network layer behaves as expected after a hard spoon.
// All network related hard spooning requirements can be supported via libp2p directly,
// without needing any additional support in Flow.
// These tests are just to reconfirm the network behaviour and provide a test bed for future tests for hard-spooning, if needed
type HardSpooningTestSuite struct {
	LibP2PNodeTestSuite
}


func TestHardSpooningTestSuite(t *testing.T) {
	suite.Run(t, new(HardSpooningTestSuite))
}

// TestNetworkKeyChangedAfterHardSpoon tests that a node from the old chain cannot talk to a node in the new chain
// The test assumes that if a node is moved from the old chain to the new chain, it will have a new network key
func (h *HardSpooningTestSuite) TestNetworkKeyChangedAfterHardSpoon() {

	// create and start node 1 on localhost and random port
	node1key, err := generateNetworkingKey("abc")
	assert.NoError(h.T(), err)
	node1, address1 := h.CreateNode("node1",  node1key, "0.0.0.0", "0")
	defer h.StopNode(node1)
	h.T().Logf(" %s node started on %s:%s", node1.name, address1.IP, address1.Port)
	h.T().Logf("libp2p ID for %s: %s", node1.name, node1.libP2PHost.ID())

	// create and start node 2 on localhost and random port
	node2key, err := generateNetworkingKey("def")
	assert.NoError(h.T(), err)
	node2, address2 := h.CreateNode("node2", node2key, "0.0.0.0", "0")
	h.T().Logf(" %s node started on %s:%s", node2.name, address2.IP, address2.Port)
	h.T().Logf("libp2p ID for %s: %s", node2.name, node2.libP2PHost.ID())

	// create stream from node 1 to node 2
	s, err := node1.CreateStream(context.Background(), address2)
	assert.NoError(h.T(), err)
	assert.NotNil(h.T(), s)

	// Simulate a hard-spoon: node1 is on the old chain, but node2 is moved from the old chain to the new chain

	// stop node 2 and start it again with a different networking key but on the same IP and port
	h.StopNode(node2)

	// generate a new key
	node2keyNew, err := generateNetworkingKey("ghi")
	assert.NoError(h.T(), err)
	assert.False(h.T(), node2key.Equals(node2keyNew))

	// start node2 with the same name, ip and port but with the new key
	node2, address2New := h.CreateNode(node2.name, node2keyNew, address2.IP, address2.Port)
	defer h.StopNode(node2)
	h.T().Logf(" %s node again started on %s:%s", node2.name, address2New.IP, address2New.Port)
	h.T().Logf("new libp2p ID for %s: %s", node2.name, node2.libP2PHost.ID())

	// make sure the node2 indeed came up on the old ip and port
	assert.Equal(h.T(), address2.IP, address2New.IP)
	assert.Equal(h.T(), address2.Port, address2New.Port)

	// attempt to create a stream from node 1 (old chain) to node 2 (new chain)
	// this time it should fail since node 2 is using a different public key
	// (and therefore has a different libp2p node id)
	_, err = node1.CreateStream(context.Background(), address2)
	// assert that stream creation failed
	assert.Error(h.T(), err)
	// assert that it failed with the expected error
	assert.Regexp(h.T(), ".*failed to negotiate security protocol: connected to wrong peer.*", err)
}
