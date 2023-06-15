package scoring_test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	mocktestify "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
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

// AddInvCtrlMsgNotifConsumer adds a consumer for invalid control message notifications.
// In this mock implementation, the consumer is stored in the mockInspectorSuite, and is used to simulate the reception of invalid control messages.
// Args:
// - c: the consumer to add.
// Returns:
// - nil.
// Note: this function will fail the test if the consumer is already set.
func (m *mockInspectorSuite) AddInvCtrlMsgNotifConsumer(c p2p.GossipSubInvCtrlMsgNotifConsumer) {
	require.Nil(m.t, m.consumer)
	m.consumer = c
}

func (m *mockInspectorSuite) Inspectors() []p2p.GossipSubRPCInspector {
	return []p2p.GossipSubRPCInspector{}
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
	node1, id1 := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		idProvider,
		p2ptest.WithRole(flow.RoleConsensus),
		p2ptest.WithPeerScoringEnabled(idProvider),
		p2ptest.WithGossipSubRpcInspectorSuite(inspectorSuite1))

	node2, id2 := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		idProvider,
		p2ptest.WithRole(flow.RoleConsensus),
		p2ptest.WithPeerScoringEnabled(idProvider))

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
	p2ptest.StartNodes(t, signalerCtx, nodes, 100*time.Millisecond)
	defer p2ptest.StopNodes(t, nodes, cancel, 2*time.Second)

	p2ptest.LetNodesDiscoverEachOther(t, ctx, nodes, ids)

	// checks end-to-end message delivery works on GossipSub
	p2ptest.EnsurePubsubMessageExchange(t, ctx, nodes, func() (interface{}, channels.Topic) {
		blockTopic := channels.TopicFromChannel(channels.PushBlocks, sporkId)
		return unittest.ProposalFixture(), blockTopic
	})

	// now simulates node2 spamming node1 with invalid gossipsub control messages.
	for i := 0; i < 30; i++ {
		inspectorSuite1.consumer.OnInvalidControlMessageNotification(&p2p.InvCtrlMsgNotif{
			PeerID:  node2.Host().ID(),
			MsgType: p2p.ControlMessageTypes()[rand.Intn(len(p2p.ControlMessageTypes()))],
			Count:   1,
			Err:     fmt.Errorf("invalid control message"),
		})
	}

	// checks no GossipSub message exchange should no longer happen between node1 and node2.
	p2ptest.EnsureNoPubsubExchangeBetweenGroups(t, ctx, []p2p.LibP2PNode{node1}, []p2p.LibP2PNode{node2}, func() (interface{}, channels.Topic) {
		blockTopic := channels.TopicFromChannel(channels.PushBlocks, sporkId)
		return unittest.ProposalFixture(), blockTopic
	})
}
