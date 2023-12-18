package p2pnode_test

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/p2p"
	p2pbuilderconfig "github.com/onflow/flow-go/network/p2p/builder/config"
	"github.com/onflow/flow-go/network/p2p/connection"
	"github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestDisconnectingFromDisallowListedNode ensures that:
// (1) the node disconnects from a disallow listed node while the node is connected to other (allow listed) nodes.
// (2) new inbound or outbound connections to and from disallow-listed nodes are rejected.
// (3) When a disallow-listed node is allow-listed again, the node reconnects to it.
func TestDisconnectingFromDisallowListedNode(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	sporkID := unittest.IdentifierFixture()
	idProvider := mockmodule.NewIdentityProvider(t)

	peerIDSlice := peer.IDSlice{}
	// node 1 is the node that will be disallow-listing another node (node 2).
	node1, identity1 := p2ptest.NodeFixture(t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithPeerManagerEnabled(&p2pbuilderconfig.PeerManagerConfig{
			ConnectionPruning: true,
			UpdateInterval:    connection.DefaultPeerUpdateInterval,
			ConnectorFactory:  connection.DefaultLibp2pBackoffConnectorFactory(),
		},
			func() peer.IDSlice {
				return peerIDSlice
			}),
		p2ptest.WithConnectionGater(p2ptest.NewConnectionGater(idProvider, func(p peer.ID) error {
			// allow all the connections, except for the ones that are disallow-listed, which are determined when
			// this connection gater object queries the disallow listing oracle that will be provided to it by
			// the libp2p node. So, here, we don't need to do anything except just enabling the connection gater.
			return nil
		})))
	idProvider.On("ByPeerID", node1.ID()).Return(&identity1, true).Maybe()
	peerIDSlice = append(peerIDSlice, node1.ID())

	// node 2 is the node that will be disallow-listed by node 1.
	node2, identity2 := p2ptest.NodeFixture(t, sporkID, t.Name(), idProvider)
	idProvider.On("ByPeerID", node2.ID()).Return(&identity2, true).Maybe()
	peerIDSlice = append(peerIDSlice, node2.ID())

	// node 3 is the node that will be connected to node 1 (to ensure that node 1 is still able to connect to other nodes
	// after disallow-listing node 2).
	node3, identity3 := p2ptest.NodeFixture(t, sporkID, t.Name(), idProvider)
	idProvider.On("ByPeerID", node3.ID()).Return(&identity3, true).Maybe()
	peerIDSlice = append(peerIDSlice, node3.ID())

	nodes := []p2p.LibP2PNode{node1, node2, node3}
	ids := flow.IdentityList{&identity1, &identity2, &identity3}

	p2ptest.StartNodes(t, signalerCtx, nodes)
	defer p2ptest.StopNodes(t, nodes, cancel)

	p2ptest.LetNodesDiscoverEachOther(t, ctx, nodes, ids)

	// initially all nodes should be connected to each other.
	p2ptest.RequireConnectedEventually(t, nodes, 100*time.Millisecond, 2*time.Second)

	// phase-1: node 1 disallow-lists node 2.
	node1.OnDisallowListNotification(node2.ID(), network.DisallowListedCauseAlsp)

	// eventually node 1 should be disconnected from node 2 while other nodes should remain connected.
	// we choose a timeout of 2 seconds because peer manager updates peers every 1 second.
	p2ptest.RequireEventuallyNotConnected(t, []p2p.LibP2PNode{node1}, []p2p.LibP2PNode{node2}, 100*time.Millisecond, 2*time.Second)

	// but nodes 1 and 3 should remain connected as well as nodes 2 and 3.
	// we choose a short timeout because we expect the nodes to remain connected.
	p2ptest.RequireConnectedEventually(t, []p2p.LibP2PNode{node1, node3}, 1*time.Millisecond, 100*time.Millisecond)
	p2ptest.RequireConnectedEventually(t, []p2p.LibP2PNode{node2, node3}, 1*time.Millisecond, 100*time.Millisecond)

	// while node 2 is disallow-listed, it cannot connect to node 1. Also, node 1 cannot directly dial and connect to node 2, unless
	// it is allow-listed again.
	p2ptest.EnsureNotConnectedBetweenGroups(t, ctx, []p2p.LibP2PNode{node1}, []p2p.LibP2PNode{node2})

	// phase-2: now we allow-list node 1 back
	node1.OnAllowListNotification(node2.ID(), network.DisallowListedCauseAlsp)

	// eventually node 1 should be connected to node 2 again, hence all nodes should be connected to each other.
	// we choose a timeout of 5 seconds because peer manager updates peers every 1 second and we need to wait for
	// any potential random backoffs to expire (min 1 second).
	p2ptest.RequireConnectedEventually(t, nodes, 100*time.Millisecond, 5*time.Second)
}
