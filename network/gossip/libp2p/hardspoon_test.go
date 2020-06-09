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
// Hard-spooning can be supported by two ways:
// 1. Updating the network key of a node after it is moved from the old chain to the new chain
// 2. Updating the Flow Libp2p protocol ID in the code
// 1 and 2 both can independently ensure that nodes from the old chain cannot communicate with nodes on the new chain
// These tests are just to reconfirm the network behaviour and provide a test bed for future tests for hard-spooning, if needed
type HardSpooningTestSuite struct {
	LibP2PNodeTestSuite
}

func TestHardSpooningTestSuite(t *testing.T) {
	suite.Run(t, new(HardSpooningTestSuite))
}

// TestNetworkKeyChangedAfterHardSpoon tests that a node from the old chain cannot talk to a node in the new chain if it's
// network key is updated even if the libp2p protocol id remains the same
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


// TestProtocolChangeAfterHardSpoon tests that a node from the old chain cannot talk to a node in the new chain if
// the Flow libp2p protocol ID is updated but the network keys are kept the same.
func (h *HardSpooningTestSuite) TestProtocolChangeAfterHardSpoon() {

	// set protocol id to the current default
	flowLibP2PProtocolID = FlowLibP2PProtocolIDVersion1

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

	// create stream from node 2 to node 1
	s, err := node2.CreateStream(context.Background(), address1)
	assert.NoError(h.T(), err)
	assert.NotNil(h.T(), s)

	// Simulate a hard-spoon: node1 is on the old chain, but node2 is moved from the old chain to the new chain

	// stop node 2 and start it again with a different libp2p protocol id to listen for
	h.StopNode(node2)

	// update the flow libp2p protocol id for node2. node1 is still listening on the old protocol
	flowLibP2PProtocolID = "/flow/push/2"

	// start node2 with the same name, ip and port but with the new key
	node2, address2New := h.CreateNode(node2.name, node2key, address2.IP, address2.Port)
	defer h.StopNode(node2)
	h.T().Logf(" %s node again started on %s:%s", node2.name, address2New.IP, address2New.Port)
	h.T().Logf("new libp2p ID for %s: %s", node2.name, node2.libP2PHost.ID())

	// make sure the node2 indeed came up on the old ip and port
	assert.Equal(h.T(), address2.IP, address2New.IP)
	assert.Equal(h.T(), address2.Port, address2New.Port)

	// attempt to create a stream from node 2 (new chain) to node 1 (old chain)
	// this time it should fail since node 2 is listening on a different protocol
	_, err = node2.CreateStream(context.Background(), address1)
	// assert that stream creation failed
	assert.Error(h.T(), err)
	// assert that it failed with the expected error
	assert.Regexp(h.T(), ".*protocol not supported.*", err)
}
