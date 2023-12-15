package scoring_test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"
	mocktestify "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/config"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/mock"
	flownet "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	config2 "github.com/onflow/flow-go/network/p2p/config"
	p2pmsg "github.com/onflow/flow-go/network/p2p/message"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/utils/unittest"
)

// mockInspectorSuite is a mock implementation of the GossipSubInspectorSuite interface.
// It is used to test the impact of invalid control messages on the scoring and connectivity of nodes in a network.
type mockInspectorSuite struct {
	component.Component
	t        *testing.T
	consumer p2p.GossipSubInvCtrlMsgNotifConsumer
}

// ensures that mockInspectorSuite implements the GossipSubInspectorSuite interface.
var _ p2p.GossipSubInspectorSuite = (*mockInspectorSuite)(nil)

func (m *mockInspectorSuite) AddInvalidControlMessageConsumer(consumer p2p.GossipSubInvCtrlMsgNotifConsumer) {
	require.Nil(m.t, m.consumer)
	m.consumer = consumer
}
func (m *mockInspectorSuite) ActiveClustersChanged(_ flow.ChainIDList) {
	// no-op
}

// newMockInspectorSuite creates a new mockInspectorSuite.
// Args:
// - t: the test object used for assertions.
// Returns:
// - a new mockInspectorSuite.
func newMockInspectorSuite(t *testing.T) *mockInspectorSuite {
	i := &mockInspectorSuite{
		t: t,
	}

	builder := component.NewComponentManagerBuilder()
	builder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
		ready()
		<-ctx.Done()
	})

	i.Component = builder.Build()
	return i
}

// InspectFunc returns a function that is called when a node receives a control message.
// In this mock implementation, the function does nothing.
func (m *mockInspectorSuite) InspectFunc() func(peer.ID, *pubsub.RPC) error {
	return nil
}

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

	inspectorSuite1 := newMockInspectorSuite(t)
	factory := func(
		irrecoverable.SignalerContext,
		zerolog.Logger,
		flow.Identifier,
		*config2.RpcInspectorParameters,
		module.GossipSubMetrics,
		metrics.HeroCacheMetricsFactory,
		flownet.NetworkingType,
		module.IdentityProvider,
		func() p2p.TopicProvider) (p2p.GossipSubInspectorSuite, error) {
		// override the gossipsub rpc inspector suite factory to return the mock inspector suite
		return inspectorSuite1, nil
	}

	cfg, err := config.DefaultConfig()
	require.NoError(t, err)

	cfg.NetworkConfig.GossipSub.ScoringParameters.AppSpecificScore.ScoreTTL = 10 * time.Millisecond // speed up the test

	node1, id1 := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		idProvider,
		p2ptest.WithRole(flow.RoleConsensus),
		p2ptest.OverrideFlowConfig(cfg),
		p2ptest.OverrideGossipSubRpcInspectorSuiteFactory(factory))

	node2, id2 := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		idProvider,
		p2ptest.WithRole(flow.RoleConsensus),
		p2ptest.OverrideFlowConfig(cfg))

	ids := flow.IdentityList{&id1, &id2}
	nodes := []p2p.LibP2PNode{node1, node2}

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
		inspectorSuite1.consumer.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
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
