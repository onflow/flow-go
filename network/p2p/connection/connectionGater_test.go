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
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/internal/p2pfixtures"
	"github.com/onflow/flow-go/network/internal/testutils"
	"github.com/onflow/flow-go/network/p2p"
	mockp2p "github.com/onflow/flow-go/network/p2p/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestConnectionGater_Lifecycle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	sporkId := unittest.IdentifierFixture()
	defer cancel()

	count := 5
	nodes := make([]p2p.LibP2PNode, 0, count)
	inbounds := make([]chan string, 0, count)

	disallowedPeerIds := map[peer.ID]struct{}{}
	allPeerIds := make(peer.IDSlice, 0, count)

	connectionGater := mockp2p.NewConnectionGater(t)
	for i := 0; i < count; i++ {
		handler, inbound := p2pfixtures.StreamHandlerFixture(t)
		node, _ := p2pfixtures.NodeFixture(
			t,
			sporkId,
			t.Name(),
			p2pfixtures.WithRole(flow.RoleConsensus),
			p2pfixtures.WithDefaultStreamHandler(handler),
			// enable peer manager, with a 1-second refresh rate, and connection pruning enabled.
			p2pfixtures.WithPeerManagerEnabled(true, 1*time.Second, func() peer.IDSlice {
				list := make(peer.IDSlice, 0)
				for _, pid := range allPeerIds {
					if _, ok := disallowedPeerIds[pid]; !ok {
						list = append(list, pid)
					}
				}
				return list
			}),
			p2pfixtures.WithConnectionGater(connectionGater))

		nodes = append(nodes, node)
		allPeerIds = append(allPeerIds, node.Host().ID())
		inbounds = append(inbounds, inbound)
	}

	connectionGater.On("InterceptSecured", mock.Anything, mock.Anything, mock.Anything).
		Return(func(_ network.Direction, p peer.ID, _ network.ConnMultiaddrs) bool {
			_, ok := disallowedPeerIds[p]
			return !ok
		})

	connectionGater.On("InterceptPeerDial", mock.Anything).Return(func(p peer.ID) bool {
		_, ok := disallowedPeerIds[p]
		return !ok
	})

	connectionGater.On("InterceptAddrDial", mock.Anything, mock.Anything).Return(true)
	connectionGater.On("InterceptAccept", mock.Anything).Return(true)

	// blacklists the first node
	disallowedPeerIds[nodes[0].Host().ID()] = struct{}{}

	// starts the nodes
	p2pfixtures.StartNodes(t, signalerCtx, nodes, 1*time.Second)
	defer p2pfixtures.StopNodes(t, nodes, cancel, 1*time.Second)

	ensureCommunicationSilenceAmongGroups(t, ctx, sporkId, nodes[:1], nodes[1:])

	// Checks that only the allowed nodes can establish an upgradable connection.
	// We intentionally mock this after checking for communication silence.
	// As no connection to/from a disallowed node should ever reach the upgradable connection stage.
	connectionGater.On("InterceptUpgraded", mock.Anything).Run(func(args mock.Arguments) {
		conn, ok := args.Get(0).(network.Conn)
		require.True(t, ok)

		remote := conn.RemotePeer()
		_, disallowed := disallowedPeerIds[remote]
		require.False(t, disallowed)

		local := conn.LocalPeer()
		_, disallowed = disallowedPeerIds[local]
		require.False(t, disallowed)
	}).Return(true, control.DisconnectReason(0))

	ensureCommunicationOverAllProtocols(t, ctx, sporkId, nodes[1:], inbounds[1:])
}

func TestConnectionGater_Disallow_Integration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	sporkId := unittest.IdentifierFixture()
	defer cancel()

	count := 5
	nodes := make([]p2p.LibP2PNode, 0, 5)
	ids := flow.IdentityList{}
	inbounds := make([]chan string, 0, 5)

	blacklist := map[*flow.Identity]struct{}{}

	for i := 0; i < count; i++ {
		handler, inbound := p2pfixtures.StreamHandlerFixture(t)
		node, id := p2pfixtures.NodeFixture(
			t,
			sporkId,
			t.Name(),
			p2pfixtures.WithRole(flow.RoleConsensus),
			p2pfixtures.WithDefaultStreamHandler(handler),
			// enable peer manager, with a 1-second refresh rate, and connection pruning enabled.
			p2pfixtures.WithPeerManagerEnabled(true, 1*time.Second, func() peer.IDSlice {
				list := make(peer.IDSlice, 0)
				for _, id := range ids {
					if _, ok := blacklist[id]; ok {
						continue
					}

					pid, err := unittest.PeerIDFromFlowID(id)
					require.NoError(t, err)

					list = append(list, pid)
				}
				return list
			}),
			p2pfixtures.WithConnectionGater(testutils.NewConnectionGater(func(pid peer.ID) error {
				for id := range blacklist {
					bid, err := unittest.PeerIDFromFlowID(id)
					require.NoError(t, err)
					if bid == pid {
						return fmt.Errorf("blacklisted")
					}
				}

				return nil
			})))

		nodes = append(nodes, node)
		ids = append(ids, &id)
		inbounds = append(inbounds, inbound)
	}

	p2pfixtures.StartNodes(t, signalerCtx, nodes, 1*time.Second)
	defer p2pfixtures.StopNodes(t, nodes, cancel, 1*time.Second)

	p2pfixtures.LetNodesDiscoverEachOther(t, ctx, nodes, ids)

	// ensures that all nodes are connected to each other, and they can exchange messages over the pubsub and unicast.
	ensureCommunicationOverAllProtocols(t, ctx, sporkId, nodes, inbounds)

	// now we blacklist one of the nodes (the last node).
	blacklist[ids[len(ids)-1]] = struct{}{}
	// let peer manager prune the connections to the blacklisted node.
	time.Sleep(1 * time.Second)
	// ensures no connection, unicast, or pubsub going to or coming from the blacklisted node.
	ensureCommunicationSilenceAmongGroups(t, ctx, sporkId, nodes[:count-1], nodes[count-1:])

	// now we blacklist another node (the second last node)
	blacklist[ids[len(ids)-2]] = struct{}{}
	// let peer manager prune the connections to the blacklisted node.
	time.Sleep(1 * time.Second)
	// ensures no connection, unicast, or pubsub going to and coming from the blacklisted nodes.
	ensureCommunicationSilenceAmongGroups(t, ctx, sporkId, nodes[:count-2], nodes[count-2:])
	// ensures that all nodes are other non-black listed nodes can exchange messages over the pubsub and unicast.
	ensureCommunicationOverAllProtocols(t, ctx, sporkId, nodes[:count-2], inbounds[:count-2])
}

// ensureCommunicationSilenceAmongGroups ensures no connection, unicast, or pubsub going to or coming from between the two groups of nodes.
func ensureCommunicationSilenceAmongGroups(t *testing.T, ctx context.Context, sporkId flow.Identifier, groupA []p2p.LibP2PNode, groupB []p2p.LibP2PNode) {
	// ensures no connection, unicast, or pubsub going to the blacklisted nodes
	p2pfixtures.EnsureNotConnectedBetweenGroups(t, ctx, groupA, groupB)
	p2pfixtures.EnsureNoPubsubExchangeBetweenGroups(t, ctx, groupA, groupB, func() (interface{}, channels.Topic) {
		blockTopic := channels.TopicFromChannel(channels.PushBlocks, sporkId)
		return unittest.ProposalFixture(), blockTopic
	})
	p2pfixtures.EnsureNoStreamCreation(t, ctx, groupA, groupB)
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
