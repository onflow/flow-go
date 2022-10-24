package connection_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p/internal/p2pfixtures"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestConnectionGater(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	sporkId := unittest.IdentifierFixture()
	defer cancel()

	// We create a group of 5 nodes WITHOUT the peer scoring enabled.
	// This is to isolate the connection gater functionality from the peer scoring, and
	// ensure that peers are truly being blocked by the connection gater (and not by the peer scoring).
	blacklistedPeers := make(map[peer.ID]struct{})
	nodes, ids := p2pfixtures.NodesFixture(t, sporkId, t.Name(), 5, p2pfixtures.WithRole(flow.RoleConsensus), p2pfixtures.WithPeerFilter(func(pid peer.ID) error {
		_, blacklisted := blacklistedPeers[pid]
		if blacklisted {
			return fmt.Errorf("peer id blacklisted: %s", pid.String())
		}
		return nil
	}))

	p2pfixtures.StartNodes(t, signalerCtx, nodes, 100*time.Millisecond)
	defer p2pfixtures.StopNodes(t, nodes, cancel, 100*time.Millisecond)

	p2pfixtures.LetNodesDiscoverEachOther(t, ctx, nodes, ids)

	// ensures that all nodes are connected to each other, and they can exchange messages over the pubsub and unicast
	p2pfixtures.EnsureConnected(t, ctx, nodes)
	p2pfixtures.EnsurePubsubMessageExchange(t, ctx, nodes, func() (interface{}, channels.Topic) {
		blockTopic := channels.TopicFromChannel(channels.PushBlocks, sporkId)
		return unittest.ProposalFixture(), blockTopic
	})
}
