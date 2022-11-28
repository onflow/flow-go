package corruptlibp2p_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/insecure/corruptlibp2p"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/p2p"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestSpam(t *testing.T) {
	sporkId := unittest.IdentifierFixture()

	var spammerAdapter p2p.PubSubAdapter
	spammerNode, _ := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		p2ptest.WithGossipSub(corruptlibp2p.CorruptibleGossipSubFactory(func(adapter p2p.PubSubAdapter) { spammerAdapter = adapter }), corruptlibp2p.CorruptibleGossipSubConfigFactory()),
	)

	victimNode, victimId := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		p2ptest.WithGossipSub(corruptlibp2p.CorruptibleGossipSubFactory(func(adapter p2p.PubSubAdapter) {}), corruptlibp2p.CorruptibleGossipSubConfigFactory()),
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
	require.NotNil(t, spammerAdapter)
	corruptAdapter, ok := spammerAdapter.(*corruptlibp2p.CorruptGossipSubAdapter)
	require.True(t, ok)

	spammer := corruptlibp2p.NewGossipSubSpammer(corruptAdapter.GetRouter())

	// start spamming the first peer
	spammer.SpamIHave(victimPeerId, 10, 1)
}
