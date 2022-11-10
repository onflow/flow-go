package corruptlibp2p

import (
	"fmt"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestSpammerGossipSub(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	sporkId := unittest.IdentifierFixture()
	defer cancel()

	count := 5
	nodes := make([]p2p.LibP2PNode, 0, 5)
	ids := flow.IdentityList{}
	inbounds := make([]chan string, 0, 5)

	disallowedList := map[*flow.Identity]struct{}{}

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
					if _, ok := disallowedList[id]; ok {
						continue
					}

					pid, err := unittest.PeerIDFromFlowID(id)
					require.NoError(t, err)

					list = append(list, pid)
				}
				return list
			}),
			p2pfixtures.WithConnectionGater(testutils.NewConnectionGater(func(pid peer.ID) error {
				for id := range disallowedList {
					bid, err := unittest.PeerIDFromFlowID(id)
					require.NoError(t, err)
					if bid == pid {
						return fmt.Errorf("disallow-listed")
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

	// now we add one of the nodes (the last node) to the disallow-list.
	disallowedList[ids[len(ids)-1]] = struct{}{}
	// let peer manager prune the connections to the disallow-listed node.
	time.Sleep(1 * time.Second)
	// ensures no connection, unicast, or pubsub going to or coming from the disallow-listed node.
	ensureCommunicationSilenceAmongGroups(t, ctx, sporkId, nodes[:count-1], nodes[count-1:])

	// now we add another node (the second last node) to the disallowed list.
	disallowedList[ids[len(ids)-2]] = struct{}{}
	// let peer manager prune the connections to the disallow-listed node.
	time.Sleep(1 * time.Second)
	// ensures no connection, unicast, or pubsub going to and coming from the disallow-listed nodes.
	ensureCommunicationSilenceAmongGroups(t, ctx, sporkId, nodes[:count-2], nodes[count-2:])
	// ensures that all nodes are other non-disallow-listed nodes can exchange messages over the pubsub and unicast.
	ensureCommunicationOverAllProtocols(t, ctx, sporkId, nodes[:count-2], inbounds[:count-2])
}
