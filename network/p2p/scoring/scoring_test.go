package scoring_test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"
	mocktestify "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/config"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/mock"
	flownet "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	p2pconfig "github.com/onflow/flow-go/network/p2p/config"
	p2pmsg "github.com/onflow/flow-go/network/p2p/message"
	mockp2p "github.com/onflow/flow-go/network/p2p/mock"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestInvalidCtrlMsgScoringIntegration tests the impact of invalid control messages on the scoring and connectivity of nodes in a network.
// It creates a network of 2 nodes, and sends a set of control messages with invalid topic IDs to one of the nodes.
// It then checks that the node receiving the invalid control messages decreases its score for the peer spamming the invalid messages, and
// eventually disconnects from the spamming peer on the gossipsub layer, i.e., messages sent by the spamming peer are no longer
// received by the node.
func TestInvalidCtrlMsgScoringIntegration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	sporkId := unittest.IdentifierFixture()
	idProvider := mock.NewIdentityProvider(t)

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	cfg.NetworkConfig.GossipSub.ScoringParameters.ScoringRegistryParameters.AppSpecificScore.ScoreTTL = 10 * time.Millisecond // speed up the test

	var notificationConsumer p2p.GossipSubInvCtrlMsgNotifConsumer
	inspector := mockp2p.NewGossipSubRPCInspector(t)
	inspector.On("Inspect", mocktestify.Anything, mocktestify.Anything).Return(nil) // no-op for the inspector
	inspector.On("ActiveClustersChanged", mocktestify.Anything).Return().Maybe()    // no-op for the inspector
	inspector.On("Start", mocktestify.Anything).Return(nil)                         // no-op for the inspector

	// mocking the Ready and Done channels to be closed
	done := make(chan struct{})
	close(done)
	f := func() <-chan struct{} {
		return done
	}
	inspector.On("Ready").Return(f()) // no-op for the inspector
	inspector.On("Done").Return(f())  // no-op for the inspector
	node1, id1 := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		idProvider,
		p2ptest.WithRole(flow.RoleConsensus),
		p2ptest.OverrideFlowConfig(cfg),
		p2ptest.OverrideGossipSubRpcInspectorFactory(func(logger zerolog.Logger,
			_ flow.Identifier,
			_ *p2pconfig.RpcInspectorParameters,
			_ module.GossipSubMetrics,
			_ metrics.HeroCacheMetricsFactory,
			_ flownet.NetworkingType,
			_ module.IdentityProvider,
			_ func() p2p.TopicProvider,
			consumer p2p.GossipSubInvCtrlMsgNotifConsumer) (p2p.GossipSubRPCInspector, error) {
			// short-wire the consumer
			notificationConsumer = consumer
			return inspector, nil
		}))

	node2, id2 := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		idProvider,
		p2ptest.WithRole(flow.RoleConsensus),
		p2ptest.OverrideFlowConfig(cfg))

	ids := flow.IdentityList{&id1, &id2}
	nodes := []p2p.LibP2PNode{node1, node2}
	// suppressing "peers provider not set error"
	p2ptest.RegisterPeerProviders(t, nodes)

	provider := id.NewFixedIdentityProvider(ids)
	idProvider.On("ByPeerID", mocktestify.Anything).Return(
		func(peerId peer.ID) *flow.Identity {
			identity, _ := provider.ByPeerID(peerId)
			return identity
		}, func(peerId peer.ID) bool {
			_, ok := provider.ByPeerID(peerId)
			return ok
		})
	p2ptest.StartNodes(t, signalerCtx, nodes)
	defer p2ptest.StopNodes(t, nodes, cancel)

	p2ptest.LetNodesDiscoverEachOther(t, ctx, nodes, ids)
	blockTopic := channels.TopicFromChannel(channels.PushBlocks, sporkId)
	// checks end-to-end message delivery works on GossipSub.
	p2ptest.EnsurePubsubMessageExchange(t, ctx, nodes, blockTopic, 1, func() interface{} {
		return unittest.ProposalFixture()
	})

	// simulates node2 spamming node1 with invalid gossipsub control messages until node2 gets dissallow listed.
	// since the decay will start lower than .99 and will only be incremented by default .01, we need to spam a lot of messages so that the node gets disallow listed
	for i := 0; i < 750; i++ {
		notificationConsumer.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
			PeerID:  node2.ID(),
			MsgType: p2pmsg.ControlMessageTypes()[rand.Intn(len(p2pmsg.ControlMessageTypes()))],
			Error:   fmt.Errorf("invalid control message"),
		})
	}

	time.Sleep(1 * time.Second) // wait for app-specific score to be updated in the cache (remember that we need at least 100 ms for the score to be updated (ScoreTTL))

	// checks no GossipSub message exchange should no longer happen between node1 and node2.
	p2ptest.EnsureNoPubsubExchangeBetweenGroups(
		t,
		ctx,
		[]p2p.LibP2PNode{node1},
		flow.IdentifierList{id1.NodeID},
		[]p2p.LibP2PNode{node2},
		flow.IdentifierList{id2.NodeID},
		blockTopic,
		1,
		func() interface{} {
			return unittest.ProposalFixture()
		})
}
