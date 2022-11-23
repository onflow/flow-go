package corruptlibp2p

import (
	"context"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/p2p"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/utils/unittest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGetGossipSubParams(t *testing.T) {
	gossipSubParams := getGossipSubParams()

	require.Equal(t, gossipSubParams, gossipSubParams)
}

func TestSpam(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	sporkId := unittest.IdentifierFixture()
	defer cancel()

	count := 5
	nodes := make([]p2p.LibP2PNode, 0, 5)
	ids := flow.IdentityList{}
	inbounds := make([]chan string, 0, 5)
	peerIds := make([]peer.ID, 5)

	for i := 0; i < count; i++ {
		handler, inbound := p2ptest.StreamHandlerFixture(t)
		node, id := p2ptest.NodeFixture(
			t,
			sporkId,
			t.Name(),
			p2ptest.WithRole(flow.RoleConsensus),
			p2ptest.WithDefaultStreamHandler(handler),
		)
		peerId, err := unittest.PeerIDFromFlowID(&id)
		require.NoError(t, err)

		nodes = append(nodes, node)
		ids = append(ids, &id)
		inbounds = append(inbounds, inbound)
		peerIds = append(peerIds, peerId)
	}

	p2ptest.StartNodes(t, signalerCtx, nodes, 1*time.Second)
	defer p2ptest.StopNodes(t, nodes, cancel, 1*time.Second)

	//// create new spammer
	//gsr := internal.NewGossipSubRouterFixture()
	//spammer := NewSpammerGossipSub(gsr.Router)
	//
	//// start spamming the first peer
	//spammer.SpamIHave(peerIds[0], 10, 1)
}
