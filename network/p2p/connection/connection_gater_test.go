package connection_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/control"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/internal/p2pfixtures"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/connection"
	mockp2p "github.com/onflow/flow-go/network/p2p/mock"
	p2pconfig "github.com/onflow/flow-go/network/p2p/p2pbuilder/config"
	"github.com/onflow/flow-go/network/p2p/p2plogging"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/network/p2p/unicast/stream"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestConnectionGating tests node allow listing by peer ID.
func TestConnectionGating(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	sporkID := unittest.IdentifierFixture()
	idProvider := mockmodule.NewIdentityProvider(t)
	// create 2 nodes
	node1Peers := unittest.NewProtectedMap[peer.ID, struct{}]()
	node1, node1Id := p2ptest.NodeFixture(
		t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithConnectionGater(p2ptest.NewConnectionGater(idProvider, func(p peer.ID) error {
			if !node1Peers.Has(p) {
				return fmt.Errorf("id not found: %s", p2plogging.PeerId(p))
			}
			return nil
		})))
	idProvider.On("ByPeerID", node1.ID()).Return(&node1Id, true).Maybe()

	node2Peers := unittest.NewProtectedMap[peer.ID, struct{}]()
	node2, node2Id := p2ptest.NodeFixture(
		t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithConnectionGater(p2ptest.NewConnectionGater(idProvider, func(p peer.ID) error {
			if !node2Peers.Has(p) {
				return fmt.Errorf("id not found: %s", p2plogging.PeerId(p))
			}
			return nil
		})))
	idProvider.On("ByPeerID", node2.ID()).Return(&node2Id, true).Maybe()

	nodes := []p2p.LibP2PNode{node1, node2}
	ids := flow.IdentityList{&node1Id, &node2Id}
	p2ptest.StartNodes(t, signalerCtx, nodes)
	defer p2ptest.StopNodes(t, nodes, cancel)

	p2pfixtures.AddNodesToEachOthersPeerStore(t, nodes, ids)

	t.Run("outbound connection to a disallowed node is rejected", func(t *testing.T) {
		// although nodes have each other addresses, they are not in the allow-lists of each other.
		// so they should not be able to connect to each other.
		p2pfixtures.EnsureNoStreamCreationBetweenGroups(t, ctx, []p2p.LibP2PNode{node1}, []p2p.LibP2PNode{node2}, func(t *testing.T, err error) {
			require.Truef(t, stream.IsErrGaterDisallowedConnection(err), "expected ErrGaterDisallowedConnection, got: %v", err)
		})
	})

	t.Run("inbound connection from an allowed node is rejected", func(t *testing.T) {
		// for an inbound connection to be established both nodes should be in each other's allow-lists.
		// the connection gater on the dialing node is checking the allow-list upon dialing.
		// the connection gater on the listening node is checking the allow-list upon accepting the connection.

		// add node2 to node1's allow list, but not the other way around.
		node1Peers.Add(node2.ID(), struct{}{})

		// from node2 -> node1 should also NOT work, since node 1 is not in node2's allow list for dialing!
		p2pfixtures.EnsureNoStreamCreation(t, ctx, []p2p.LibP2PNode{node2}, []p2p.LibP2PNode{node1}, func(t *testing.T, err error) {
			// dialing node-1 by node-2 should fail locally at the connection gater of node-2.
			require.Truef(t, stream.IsErrGaterDisallowedConnection(err), "expected ErrGaterDisallowedConnection, got: %v", err)
		})

		// now node2 should be able to connect to node1.
		// from node1 -> node2 shouldn't work
		p2pfixtures.EnsureNoStreamCreation(t, ctx, []p2p.LibP2PNode{node1}, []p2p.LibP2PNode{node2})
	})

	t.Run("outbound connection to an approved node is allowed", func(t *testing.T) {
		// adding both nodes to each other's allow lists.
		node1Peers.Add(node2.ID(), struct{}{})
		node2Peers.Add(node1.ID(), struct{}{})

		// now both nodes should be able to connect to each other.
		p2ptest.EnsureStreamCreationInBothDirections(t, ctx, []p2p.LibP2PNode{node1, node2})
	})
}

// TestConnectionGating_ResourceAllocation_AllowListing tests resource allocation when a connection from an allow-listed node is established.
// The test directly mocks the underlying resource manager metrics of the libp2p native resource manager to ensure that the
// expected set of resources are allocated for the connection upon establishment.
func TestConnectionGating_ResourceAllocation_AllowListing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	sporkID := unittest.IdentifierFixture()
	idProvider := mockmodule.NewIdentityProvider(t)

	node1, node1Id := p2ptest.NodeFixture(
		t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithRole(flow.RoleConsensus))

	node2Metrics := mockmodule.NewNetworkMetrics(t)
	// libp2p native resource manager metrics:
	// we expect exactly 1 connection to be established from node1 to node2 (inbound for node 2).
	node2Metrics.On("AllowConn", network.DirInbound, true).Return().Once()
	// we expect the libp2p.identify service to be used to establish the connection.
	node2Metrics.On("AllowService", "libp2p.identify").Return()
	// we expect the node2 attaching node1 to the incoming connection.
	node2Metrics.On("AllowPeer", node1.ID()).Return()
	// we expect node2 allocate memory for the incoming connection.
	node2Metrics.On("AllowMemory", mock.Anything)
	// we expect node2 to allow the stream to be created.
	node2Metrics.On("AllowStream", node1.ID(), mock.Anything)
	// we expect node2 to attach protocol to the created stream.
	node2Metrics.On("AllowProtocol", mock.Anything).Return()

	// Flow-level resource allocation metrics:
	// We expect both of the following to be called as they are called together in the same function.
	node2Metrics.On("InboundConnections", mock.Anything).Return()
	node2Metrics.On("OutboundConnections", mock.Anything).Return()

	// Libp2p control message validation metrics, these may or may not be called depending on the machine the test is running on and how long
	// the nodes in the test run for.
	node2Metrics.On("BlockingPreProcessingStarted", mock.Anything, mock.Anything).Maybe()
	node2Metrics.On("BlockingPreProcessingFinished", mock.Anything, mock.Anything, mock.Anything).Maybe()
	node2Metrics.On("AsyncProcessingStarted", mock.Anything).Maybe()
	node2Metrics.On("AsyncProcessingFinished", mock.Anything, mock.Anything).Maybe()

	// we create node2 with a connection gater that allows all connections and the mocked metrics collector.
	node2, node2Id := p2ptest.NodeFixture(
		t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithRole(flow.RoleConsensus),
		p2ptest.WithMetricsCollector(node2Metrics),
		// we use default resource manager rather than the test resource manager to ensure that the metrics are called.
		p2ptest.WithDefaultResourceManager(),
		p2ptest.WithConnectionGater(p2ptest.NewConnectionGater(idProvider, func(p peer.ID) error {
			return nil // allow all connections.
		})))
	idProvider.On("ByPeerID", node1.ID()).Return(&node1Id, true).Maybe()
	idProvider.On("ByPeerID", node2.ID()).Return(&node2Id, true).Maybe()

	nodes := []p2p.LibP2PNode{node1, node2}
	ids := flow.IdentityList{&node1Id, &node2Id}
	p2ptest.StartNodes(t, signalerCtx, nodes)
	defer p2ptest.StopNodes(t, nodes, cancel)

	p2pfixtures.AddNodesToEachOthersPeerStore(t, nodes, ids)

	// now node-1 should be able to connect to node-2.
	p2pfixtures.EnsureStreamCreation(t, ctx, []p2p.LibP2PNode{node1}, []p2p.LibP2PNode{node2})

	node2Metrics.AssertExpectations(t)
}

// TestConnectionGating_ResourceAllocation_DisAllowListing tests resource allocation when a connection from an allow-listed node is established.
func TestConnectionGating_ResourceAllocation_DisAllowListing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	sporkID := unittest.IdentifierFixture()
	idProvider := mockmodule.NewIdentityProvider(t)

	node1, node1Id := p2ptest.NodeFixture(
		t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithRole(flow.RoleConsensus))

	node2Metrics := mockmodule.NewNetworkMetrics(t)
	node2Metrics.On("AllowConn", network.DirInbound, true).Return()
	node2, node2Id := p2ptest.NodeFixture(
		t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithRole(flow.RoleConsensus),
		p2ptest.WithMetricsCollector(node2Metrics),
		// we use default resource manager rather than the test resource manager to ensure that the metrics are called.
		p2ptest.WithDefaultResourceManager(),
		p2ptest.WithConnectionGater(p2ptest.NewConnectionGater(idProvider, func(p peer.ID) error {
			return fmt.Errorf("disallowed connection") // rejecting all connections.
		})))
	idProvider.On("ByPeerID", node1.ID()).Return(&node1Id, true).Maybe()
	idProvider.On("ByPeerID", node2.ID()).Return(&node2Id, true).Maybe()

	nodes := []p2p.LibP2PNode{node1, node2}
	ids := flow.IdentityList{&node1Id, &node2Id}
	p2ptest.StartNodes(t, signalerCtx, nodes)
	defer p2ptest.StopNodes(t, nodes, cancel)

	p2pfixtures.AddNodesToEachOthersPeerStore(t, nodes, ids)

	// now node2 should be able to connect to node1.
	p2pfixtures.EnsureNoStreamCreation(t, ctx, []p2p.LibP2PNode{node1}, []p2p.LibP2PNode{node2})

	// as node-2 connection gater is rejecting all connections, none of the following resource allocation methods should be called.
	node2Metrics.AssertNotCalled(t, "AllowService", mock.Anything)               // no service is allowed, e.g., libp2p.identify
	node2Metrics.AssertNotCalled(t, "AllowPeer", mock.Anything)                  // no peer is allowed to be attached to the connection.
	node2Metrics.AssertNotCalled(t, "AllowMemory", mock.Anything)                // no memory is EVER allowed to be used during the test.
	node2Metrics.AssertNotCalled(t, "AllowStream", mock.Anything, mock.Anything) // no stream is allowed to be created.
}

// TestConnectionGater_InterceptUpgrade tests the connection gater only upgrades the connections to the allow-listed peers.
// Upgrading a connection means that the connection is the last phase of the connection establishment process.
// It means that the connection is ready to be used for sending and receiving messages.
// It checks that no disallowed peer can upgrade the connection.
func TestConnectionGater_InterceptUpgrade(t *testing.T) {
	unittest.SkipUnless(t, unittest.TEST_FLAKY, "fails locally and on CI regularly")
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	sporkId := unittest.IdentifierFixture()
	defer cancel()

	count := 5
	nodes := make([]p2p.LibP2PNode, 0, count)
	inbounds := make([]chan string, 0, count)
	identities := make(flow.IdentityList, 0, count)

	disallowedPeerIds := unittest.NewProtectedMap[peer.ID, struct{}]()
	allPeerIds := make(peer.IDSlice, 0, count)
	idProvider := mockmodule.NewIdentityProvider(t)
	connectionGater := mockp2p.NewConnectionGater(t)
	for i := 0; i < count; i++ {
		handler, inbound := p2ptest.StreamHandlerFixture(t)
		node, id := p2ptest.NodeFixture(
			t,
			sporkId,
			t.Name(),
			idProvider,
			p2ptest.WithRole(flow.RoleConsensus),
			p2ptest.WithDefaultStreamHandler(handler),
			// enable peer manager, with a 1-second refresh rate, and connection pruning enabled.
			p2ptest.WithPeerManagerEnabled(&p2pconfig.PeerManagerConfig{
				ConnectionPruning: true,
				UpdateInterval:    1 * time.Second,
				ConnectorFactory:  connection.DefaultLibp2pBackoffConnectorFactory(),
			}, func() peer.IDSlice {
				list := make(peer.IDSlice, 0)
				for _, pid := range allPeerIds {
					if !disallowedPeerIds.Has(pid) {
						list = append(list, pid)
					}
				}
				return list
			}),
			p2ptest.WithConnectionGater(connectionGater))
		idProvider.On("ByPeerID", node.ID()).Return(&id, true).Maybe()
		nodes = append(nodes, node)
		identities = append(identities, &id)
		allPeerIds = append(allPeerIds, node.ID())
		inbounds = append(inbounds, inbound)
	}

	connectionGater.On("InterceptSecured", mock.Anything, mock.Anything, mock.Anything).
		Return(func(_ network.Direction, p peer.ID, _ network.ConnMultiaddrs) bool {
			return !disallowedPeerIds.Has(p)
		})

	connectionGater.On("InterceptPeerDial", mock.Anything).Return(func(p peer.ID) bool {
		return !disallowedPeerIds.Has(p)
	})

	// we don't inspect connections during "accept" and "dial" phases as the peer IDs are not available at those phases.
	connectionGater.On("InterceptAddrDial", mock.Anything, mock.Anything).Return(true)
	connectionGater.On("InterceptAccept", mock.Anything).Return(true)

	// adds first node to disallowed list
	disallowedPeerIds.Add(nodes[0].ID(), struct{}{})

	// starts the nodes
	p2ptest.StartNodes(t, signalerCtx, nodes)
	defer p2ptest.StopNodes(t, nodes, cancel)

	// Checks that only an allowed REMOTE node can establish an upgradable connection.
	connectionGater.On("InterceptUpgraded", mock.Anything).Run(func(args mock.Arguments) {
		conn, ok := args.Get(0).(network.Conn)
		require.True(t, ok)

		// we don't check for the local peer as with v0.24 of libp2p, the local peer may be able to upgrade an empty connection
		// even though the remote peer has already disconnected and rejected the connection.
		remote := conn.RemotePeer()
		require.False(t, disallowedPeerIds.Has(remote))
	}).Return(true, control.DisconnectReason(0))

	ensureCommunicationSilenceAmongGroups(t, ctx, sporkId, nodes[:1], identities[:1].NodeIDs(), nodes[1:], identities[1:].NodeIDs())
	ensureCommunicationOverAllProtocols(t, ctx, sporkId, nodes[1:], inbounds[1:])
}

// TestConnectionGater_Disallow_Integration tests that when a peer is disallowed, it is disconnected from all other peers, and
// cannot connect, exchange unicast, or pubsub messages to any other peers.
// It also checked that the allowed peers can still communicate with each other.
func TestConnectionGater_Disallow_Integration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	sporkId := unittest.IdentifierFixture()
	idProvider := mockmodule.NewIdentityProvider(t)
	defer cancel()

	count := 5
	nodes := make([]p2p.LibP2PNode, 0, 5)
	ids := flow.IdentityList{}
	inbounds := make([]chan string, 0, 5)

	disallowedList := unittest.NewProtectedMap[*flow.Identity, struct{}]()

	for i := 0; i < count; i++ {
		handler, inbound := p2ptest.StreamHandlerFixture(t)
		node, id := p2ptest.NodeFixture(
			t,
			sporkId,
			t.Name(),
			idProvider,
			p2ptest.WithRole(flow.RoleConsensus),
			p2ptest.WithDefaultStreamHandler(handler),
			// enable peer manager, with a 1-second refresh rate, and connection pruning enabled.
			p2ptest.WithPeerManagerEnabled(&p2pconfig.PeerManagerConfig{
				ConnectionPruning: true,
				UpdateInterval:    1 * time.Second,
				ConnectorFactory:  connection.DefaultLibp2pBackoffConnectorFactory(),
			}, func() peer.IDSlice {
				list := make(peer.IDSlice, 0)
				for _, id := range ids {
					if disallowedList.Has(id) {
						continue
					}

					pid, err := unittest.PeerIDFromFlowID(id)
					require.NoError(t, err)

					list = append(list, pid)
				}
				return list
			}),
			p2ptest.WithConnectionGater(p2ptest.NewConnectionGater(idProvider, func(pid peer.ID) error {
				return disallowedList.ForEach(func(id *flow.Identity, _ struct{}) error {
					bid, err := unittest.PeerIDFromFlowID(id)
					require.NoError(t, err)
					if bid == pid {
						return fmt.Errorf("disallow-listed")
					}
					return nil
				})
			})))
		idProvider.On("ByPeerID", node.ID()).Return(&id, true).Maybe()

		nodes = append(nodes, node)
		ids = append(ids, &id)
		inbounds = append(inbounds, inbound)
	}

	p2ptest.StartNodes(t, signalerCtx, nodes)
	defer p2ptest.StopNodes(t, nodes, cancel)

	p2ptest.LetNodesDiscoverEachOther(t, ctx, nodes, ids)

	// ensures that all nodes are connected to each other, and they can exchange messages over the pubsub and unicast.
	ensureCommunicationOverAllProtocols(t, ctx, sporkId, nodes, inbounds)

	// now we add one of the nodes (the last node) to the disallow-list.
	disallowedList.Add(ids[len(ids)-1], struct{}{})
	// let peer manager prune the connections to the disallow-listed node.
	time.Sleep(1 * time.Second)
	// ensures no connection, unicast, or pubsub going to or coming from the disallow-listed node.
	ensureCommunicationSilenceAmongGroups(t, ctx, sporkId, nodes[:count-1], ids[:count-1].NodeIDs(), nodes[count-1:], ids[count-1:].NodeIDs())

	// now we add another node (the second last node) to the disallowed list.
	disallowedList.Add(ids[len(ids)-2], struct{}{})
	// let peer manager prune the connections to the disallow-listed node.
	time.Sleep(1 * time.Second)
	// ensures no connection, unicast, or pubsub going to and coming from the disallow-listed nodes.
	ensureCommunicationSilenceAmongGroups(t, ctx, sporkId, nodes[:count-2], ids[:count-2].NodeIDs(), nodes[count-2:], ids[count-2:].NodeIDs())
	// ensures that all nodes are other non-disallow-listed nodes can exchange messages over the pubsub and unicast.
	ensureCommunicationOverAllProtocols(t, ctx, sporkId, nodes[:count-2], inbounds[:count-2])
}

// ensureCommunicationSilenceAmongGroups ensures no connection, unicast, or pubsub going to or coming from between the two groups of nodes.
func ensureCommunicationSilenceAmongGroups(
	t *testing.T,
	ctx context.Context,
	sporkId flow.Identifier,
	groupANodes []p2p.LibP2PNode,
	groupAIdentifiers flow.IdentifierList,
	groupBNodes []p2p.LibP2PNode,
	groupBIdentifiers flow.IdentifierList) {
	// ensures no connection, unicast, or pubsub going to the disallow-listed nodes
	p2ptest.EnsureNotConnectedBetweenGroups(t, ctx, groupANodes, groupBNodes)

	blockTopic := channels.TopicFromChannel(channels.PushBlocks, sporkId)
	p2ptest.EnsureNoPubsubExchangeBetweenGroups(
		t,
		ctx,
		groupANodes,
		groupAIdentifiers,
		groupBNodes,
		groupBIdentifiers,
		blockTopic,
		1,
		func() interface{} {
			return unittest.ProposalFixture()
		})
	p2pfixtures.EnsureNoStreamCreationBetweenGroups(t, ctx, groupANodes, groupBNodes)
}

// ensureCommunicationOverAllProtocols ensures that all nodes are connected to each other, and they can exchange messages over the pubsub and unicast.
func ensureCommunicationOverAllProtocols(t *testing.T, ctx context.Context, sporkId flow.Identifier, nodes []p2p.LibP2PNode, inbounds []chan string) {
	blockTopic := channels.TopicFromChannel(channels.PushBlocks, sporkId)
	p2ptest.TryConnectionAndEnsureConnected(t, ctx, nodes)
	p2ptest.EnsurePubsubMessageExchange(t, ctx, nodes, blockTopic, 1, func() interface{} {
		return unittest.ProposalFixture()
	})
	p2pfixtures.EnsureMessageExchangeOverUnicast(t, ctx, nodes, inbounds, p2pfixtures.LongStringMessageFactoryFixture(t))
}
