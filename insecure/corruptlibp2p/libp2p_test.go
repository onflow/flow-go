package corruptlibp2p_test

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
	corrupt "github.com/yhassanzadeh13/go-libp2p-pubsub"

	"github.com/onflow/flow-go/insecure/corruptlibp2p"
	"github.com/onflow/flow-go/insecure/internal"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/p2p"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestSpam(t *testing.T) {
	sporkId := unittest.IdentifierFixture()

	var router *corrupt.GossipSubRouter
	factory := corruptlibp2p.CorruptibleGossipSubFactory(func(r *corrupt.GossipSubRouter) {
		require.NotNil(t, r)
		router = r // save the router at the initialization time of the factory
	})

	spammerNode, _ := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		internal.WithCorruptGossipSub(factory,
			corruptlibp2p.CorruptibleGossipSubConfigFactoryWithInspector(func(id peer.ID, rpc *corrupt.RPC) error {
				// here we can inspect the incoming RPC message to the spammer node
				return nil
			})),
	)

	victimNode, victimId := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		internal.WithCorruptGossipSub(factory,
			corruptlibp2p.CorruptibleGossipSubConfigFactoryWithInspector(func(id peer.ID, rpc *corrupt.RPC) error {
				// here we can inspect the incoming RPC message to the victim node
				return nil
			})),
	)
	victimPeerId, err := unittest.PeerIDFromFlowID(&victimId)
	require.NoError(t, err)

	// starts nodes
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	defer cancel()
	p2ptest.StartNodes(t, signalerCtx, []p2p.LibP2PNode{spammerNode, victimNode}, 100*time.Second)
	defer p2ptest.StopNodes(t, []p2p.LibP2PNode{spammerNode, victimNode}, cancel, 100*time.Second)

	// create new spammer
	spammer := corruptlibp2p.NewSpammerGossipSubRouter(router)

	// start spamming the first peer
	spammer.SpamIHave(victimPeerId, 1, 1)

	time.Sleep(5 * time.Second)
}
