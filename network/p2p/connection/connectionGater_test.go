package connection_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p/internal/p2pfixtures"
	"github.com/onflow/flow-go/network/p2p/p2pnode"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestConnectionGater(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	sporkId := unittest.IdentifierFixture()
	defer cancel()

	count := 5
	nodes := make([]*p2pnode.Node, 0, 5)
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
			p2pfixtures.WithPeerFilter(func(pid peer.ID) error {
				for id := range blacklist {
					bid, err := unittest.PeerIDFromFlowID(id)
					require.NoError(t, err)
					if bid == pid {
						return fmt.Errorf("blacklisted")
					}
				}

				return nil
			}))

		nodes = append(nodes, node)
		ids = append(ids, &id)
		inbounds = append(inbounds, inbound)
	}

	p2pfixtures.StartNodes(t, signalerCtx, nodes, 1*time.Second)
	defer p2pfixtures.StopNodes(t, nodes, cancel, 1*time.Second)

	p2pfixtures.LetNodesDiscoverEachOther(t, ctx, nodes, ids)

	// ensures that all nodes are connected to each other, and they can exchange messages over the pubsub and unicast
	p2pfixtures.EnsureConnected(t, ctx, nodes)
	p2pfixtures.EnsurePubsubMessageExchange(t, ctx, nodes, ids, func() (interface{}, channels.Topic) {
		blockTopic := channels.TopicFromChannel(channels.PushBlocks, sporkId)
		return unittest.ProposalFixture(), blockTopic
	})
	p2pfixtures.EnsureMessageExchangeOverUnicast(t, ctx, nodes, ids, inbounds, p2pfixtures.LongStringMessageFactoryFixture(t))

	p2pfixtures.LetNodesDiscoverEachOther(t, ctx, nodes, ids)

	// now we blacklist one of the nodes (the last node)
	blacklist[ids[len(ids)-1]] = struct{}{}

	// let peer manager prune the connections to the blacklisted node.
	time.Sleep(1 * time.Second)

	// ensures no connection, unicast, or pubsub going to the blacklisted node
	p2pfixtures.EnsureNotConnected(t, ctx, nodes[:count-1], nodes[count-1:])
	p2pfixtures.EnsureNoPubsubMessageExchange(t, ctx, nodes[:count-1], nodes[count-1:], func() (interface{}, channels.Topic) {
		blockTopic := channels.TopicFromChannel(channels.PushBlocks, sporkId)
		return unittest.ProposalFixture(), blockTopic
	})
	p2pfixtures.EnsureNoStreamCreation(t, ctx, nodes[:count-1], ids[:count-1], nodes[count-1:], ids[count-1:])

	// now we blacklist another node (the second last node)
	blacklist[ids[len(ids)-2]] = struct{}{}

	// let peer manager prune the connections to the blacklisted node.
	time.Sleep(1 * time.Second)

	// ensures no connection, unicast, or pubsub going to the blacklisted nodes
	p2pfixtures.EnsureNotConnected(t, ctx, nodes[:count-2], nodes[count-2:])
	p2pfixtures.EnsureNoPubsubMessageExchange(t, ctx, nodes[:count-2], nodes[count-2:], func() (interface{}, channels.Topic) {
		blockTopic := channels.TopicFromChannel(channels.PushBlocks, sporkId)
		return unittest.ProposalFixture(), blockTopic
	})
	p2pfixtures.EnsureNoStreamCreation(t, ctx, nodes[:count-2], ids[:count-2], nodes[count-2:], ids[count-2:])

	// ensures that all nodes are other non-black listed nodes are connected to each other.
	p2pfixtures.EnsureConnected(t, ctx, nodes[:count-2])
	p2pfixtures.EnsurePubsubMessageExchange(t, ctx, nodes[:count-2], ids[:count-2], func() (interface{}, channels.Topic) {
		blockTopic := channels.TopicFromChannel(channels.PushBlocks, sporkId)
		return unittest.ProposalFixture(), blockTopic
	})
	p2pfixtures.EnsureMessageExchangeOverUnicast(t, ctx, nodes[:count-2], ids[:count-2], inbounds[:count-2], p2pfixtures.LongStringMessageFactoryFixture(t))
}
