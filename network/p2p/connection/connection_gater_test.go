package connection_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/control"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/net/swarm"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/internal/p2pfixtures"
	"github.com/onflow/flow-go/network/internal/testutils"
	"github.com/onflow/flow-go/network/p2p"
	mockp2p "github.com/onflow/flow-go/network/p2p/mock"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestConnectionGating tests node allow listing by peer ID.
func TestConnectionGating(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	sporkID := unittest.IdentifierFixture()

	// create 2 nodes
	node1Peers := unittest.NewProtectedMap[peer.ID, struct{}]()
	node1, node1Id := p2ptest.NodeFixture(
		t,
		sporkID,
		t.Name(),
		p2ptest.WithConnectionGater(testutils.NewConnectionGater(func(p peer.ID) error {
			if !node1Peers.Has(p) {
				return fmt.Errorf("id not found: %s", p.String())
			}
			return nil
		})))

	node2Peers := unittest.NewProtectedMap[peer.ID, struct{}]()
	node2, node2Id := p2ptest.NodeFixture(
		t,
		sporkID,
		t.Name(),
		p2ptest.WithConnectionGater(testutils.NewConnectionGater(func(p peer.ID) error {
			if !node2Peers.Has(p) {
				return fmt.Errorf("id not found: %s", p.String())
			}
			return nil
		})))

	nodes := []p2p.LibP2PNode{node1, node2}
	ids := flow.IdentityList{&node1Id, &node2Id}
	p2ptest.StartNodes(t, signalerCtx, nodes, 100*time.Millisecond)
	defer p2ptest.StopNodes(t, nodes, cancel, 100*time.Millisecond)

	p2pfixtures.AddNodesToEachOthersPeerStore(t, nodes, ids)

	t.Run("outbound connection to a disallowed node is rejected", func(t *testing.T) {
		// although nodes have each other addresses, they are not in the allow-lists of each other.
		// so they should not be able to connect to each other.
		p2pfixtures.EnsureNoStreamCreationBetweenGroups(t, ctx, []p2p.LibP2PNode{node1}, []p2p.LibP2PNode{node2}, func(t *testing.T, err error) {
			require.True(t, errors.Is(err, swarm.ErrGaterDisallowedConnection))
		})
	})

	t.Run("inbound connection from an allowed node is rejected", func(t *testing.T) {
		// for an inbound connection to be established both nodes should be in each other's allow-lists.
		// the connection gater on the dialing node is checking the allow-list upon dialing.
		// the connection gater on the listening node is checking the allow-list upon accepting the connection.

		// add node2 to node1's allow list, but not the other way around.
		node1Peers.Add(node2.Host().ID(), struct{}{})

		// now node2 should be able to connect to node1.
		// from node1 -> node2 shouldn't work
		p2pfixtures.EnsureNoStreamCreation(t, ctx, []p2p.LibP2PNode{node1}, []p2p.LibP2PNode{node2})

		// however, from node2 -> node1 should also NOT work, since node 1 is not in node2's allow list for dialing!
		p2pfixtures.EnsureNoStreamCreation(t, ctx, []p2p.LibP2PNode{node2}, []p2p.LibP2PNode{node1})
	})

	t.Run("outbound connection to an approved node is allowed", func(t *testing.T) {
		// adding both nodes to each other's allow lists.
		node1Peers.Add(node2.Host().ID(), struct{}{})
		node2Peers.Add(node1.Host().ID(), struct{}{})

		// now both nodes should be able to connect to each other.
		p2pfixtures.EnsureStreamCreationInBothDirections(t, ctx, []p2p.LibP2PNode{node1, node2})
	})
}

// TestConnectionGater_InterceptUpgrade tests the connection gater only upgrades the connections to the allow-listed peers.
// Upgrading a connection means that the connection is the last phase of the connection establishment process.
// It means that the connection is ready to be used for sending and receiving messages.
// It checks that no disallowed peer can upgrade the connection.
func TestConnectionGater_InterceptUpgrade(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	sporkId := unittest.IdentifierFixture()
	defer cancel()

	count := 5
	nodes := make([]p2p.LibP2PNode, 0, count)
	inbounds := make([]chan string, 0, count)

	disallowedPeerIds := unittest.NewProtectedMap[peer.ID, struct{}]()
	allPeerIds := make(peer.IDSlice, 0, count)

	connectionGater := mockp2p.NewConnectionGater(t)
	for i := 0; i < count; i++ {
		handler, inbound := p2ptest.StreamHandlerFixture(t)
		node, _ := p2ptest.NodeFixture(
			t,
			sporkId,
			t.Name(),
			p2ptest.WithRole(flow.RoleConsensus),
			p2ptest.WithDefaultStreamHandler(handler),
			// enable peer manager, with a 1-second refresh rate, and connection pruning enabled.
			p2ptest.WithPeerManagerEnabled(true, 1*time.Second, func() peer.IDSlice {
				list := make(peer.IDSlice, 0)
				for _, pid := range allPeerIds {
					if !disallowedPeerIds.Has(pid) {
						list = append(list, pid)
					}
				}
				return list
			}),
			p2ptest.WithConnectionGater(connectionGater))

		nodes = append(nodes, node)
		allPeerIds = append(allPeerIds, node.Host().ID())
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
	disallowedPeerIds.Add(nodes[0].Host().ID(), struct{}{})

	// starts the nodes
	p2ptest.StartNodes(t, signalerCtx, nodes, 1*time.Second)
	defer p2ptest.StopNodes(t, nodes, cancel, 1*time.Second)

	ensureCommunicationSilenceAmongGroups(t, ctx, sporkId, nodes[:1], nodes[1:])

	// Checks that only the allowed nodes can establish an upgradable connection.
	// We intentionally mock this after checking for communication silence.
	// As no connection to/from a disallowed node should ever reach the upgradable connection stage.
	connectionGater.On("InterceptUpgraded", mock.Anything).Run(func(args mock.Arguments) {
		conn, ok := args.Get(0).(network.Conn)
		require.True(t, ok)

		remote := conn.RemotePeer()
		require.False(t, disallowedPeerIds.Has(remote))

		local := conn.LocalPeer()
		require.False(t, disallowedPeerIds.Has(local))
	}).Return(true, control.DisconnectReason(0))

	ensureCommunicationOverAllProtocols(t, ctx, sporkId, nodes[1:], inbounds[1:])
}

// TestConnectionGater_Disallow_Integration tests that when a peer is disallowed, it is disconnected from all other peers, and
// cannot connect, exchange unicast, or pubsub messages to any other peers.
// It also checked that the allowed peers can still communicate with each other.
func TestConnectionGater_Disallow_Integration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	sporkId := unittest.IdentifierFixture()
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
			p2ptest.WithRole(flow.RoleConsensus),
			p2ptest.WithDefaultStreamHandler(handler),
			// enable peer manager, with a 1-second refresh rate, and connection pruning enabled.
			p2ptest.WithPeerManagerEnabled(true, 1*time.Second, func() peer.IDSlice {
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
			p2ptest.WithConnectionGater(testutils.NewConnectionGater(func(pid peer.ID) error {
				return disallowedList.ForEach(func(id *flow.Identity, _ struct{}) error {
					bid, err := unittest.PeerIDFromFlowID(id)
					require.NoError(t, err)
					if bid == pid {
						return fmt.Errorf("disallow-listed")
					}
					return nil
				})
			})))

		nodes = append(nodes, node)
		ids = append(ids, &id)
		inbounds = append(inbounds, inbound)
	}

	p2ptest.StartNodes(t, signalerCtx, nodes, 1*time.Second)
	defer p2ptest.StopNodes(t, nodes, cancel, 1*time.Second)

	p2ptest.LetNodesDiscoverEachOther(t, ctx, nodes, ids)

	// ensures that all nodes are connected to each other, and they can exchange messages over the pubsub and unicast.
	ensureCommunicationOverAllProtocols(t, ctx, sporkId, nodes, inbounds)

	// now we add one of the nodes (the last node) to the disallow-list.
	disallowedList.Add(ids[len(ids)-1], struct{}{})
	// let peer manager prune the connections to the disallow-listed node.
	time.Sleep(1 * time.Second)
	// ensures no connection, unicast, or pubsub going to or coming from the disallow-listed node.
	ensureCommunicationSilenceAmongGroups(t, ctx, sporkId, nodes[:count-1], nodes[count-1:])

	// now we add another node (the second last node) to the disallowed list.
	disallowedList.Add(ids[len(ids)-2], struct{}{})
	// let peer manager prune the connections to the disallow-listed node.
	time.Sleep(1 * time.Second)
	// ensures no connection, unicast, or pubsub going to and coming from the disallow-listed nodes.
	ensureCommunicationSilenceAmongGroups(t, ctx, sporkId, nodes[:count-2], nodes[count-2:])
	// ensures that all nodes are other non-disallow-listed nodes can exchange messages over the pubsub and unicast.
	ensureCommunicationOverAllProtocols(t, ctx, sporkId, nodes[:count-2], inbounds[:count-2])
}

// ensureCommunicationSilenceAmongGroups ensures no connection, unicast, or pubsub going to or coming from between the two groups of nodes.
func ensureCommunicationSilenceAmongGroups(t *testing.T, ctx context.Context, sporkId flow.Identifier, groupA []p2p.LibP2PNode, groupB []p2p.LibP2PNode) {
	// ensures no connection, unicast, or pubsub going to the disallow-listed nodes
	p2pfixtures.EnsureNotConnectedBetweenGroups(t, ctx, groupA, groupB)
	p2pfixtures.EnsureNoPubsubExchangeBetweenGroups(t, ctx, groupA, groupB, func() (interface{}, channels.Topic) {
		blockTopic := channels.TopicFromChannel(channels.PushBlocks, sporkId)
		return unittest.ProposalFixture(), blockTopic
	})
	p2pfixtures.EnsureNoStreamCreationBetweenGroups(t, ctx, groupA, groupB)
}

// ensureCommunicationOverAllProtocols ensures that all nodes are connected to each other, and they can exchange messages over the pubsub and unicast.
func ensureCommunicationOverAllProtocols(t *testing.T, ctx context.Context, sporkId flow.Identifier, nodes []p2p.LibP2PNode, inbounds []chan string) {
	p2pfixtures.EnsureConnected(t, ctx, nodes)
	p2pfixtures.EnsurePubsubMessageExchange(t, ctx, nodes, func() (interface{}, channels.Topic) {
		blockTopic := channels.TopicFromChannel(channels.PushBlocks, sporkId)
		return unittest.ProposalFixture(), blockTopic
	})
	p2pfixtures.EnsureMessageExchangeOverUnicast(t, ctx, nodes, inbounds, p2pfixtures.LongStringMessageFactoryFixture(t))
}
