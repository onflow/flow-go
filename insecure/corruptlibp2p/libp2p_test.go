package corruptlibp2p_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/insecure/corruptlibp2p"
	internalinsecure "github.com/onflow/flow-go/insecure/internal"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/p2p"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestSpam(t *testing.T) {
	sporkId := unittest.IdentifierFixture()

	spammerGossipSubOpt, spammerRouter := WithCorruptGossipSub()
	spammerNode, _ := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		spammerGossipSubOpt,
	)
	// require.NotNil(t, spammerRouter)

	victimGossipSubOpt, _ := WithCorruptGossipSub()
	victimNode, victimId := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		victimGossipSubOpt,
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
	require.NotNil(t, spammerRouter)
	spammer := corruptlibp2p.NewGossipSubSpammer(spammerRouter)

	// start spamming the first peer
	spammer.SpamIHave(victimPeerId, 10, 1)
}

func WithCorruptGossipSub() (p2ptest.NodeFixtureParameterOption, *internalinsecure.CorruptGossipSubRouter) {
	factory, router := corruptlibp2p.CorruptibleGossipSubFactory()
	config := corruptlibp2p.CorruptibleGossipSubConfigFactory()
	return p2ptest.WithGossipSub(factory, config), router
}
