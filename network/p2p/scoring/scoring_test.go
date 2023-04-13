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
	"github.com/onflow/flow-go/network/internal/p2pfixtures"
	"github.com/onflow/flow-go/network/p2p"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/utils/unittest"
)

type mockInspectorSuite struct {
	component.Component
	t        *testing.T
	consumer p2p.GossipSubInvalidControlMessageNotificationConsumer
}

var _ p2p.GossipSubInspectorSuite = (*mockInspectorSuite)(nil)

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

func (m *mockInspectorSuite) InspectFunc() func(peer.ID, *pubsub.RPC) error {
	return nil
}

func (m *mockInspectorSuite) AddInvalidCtrlMsgNotificationConsumer(c p2p.GossipSubInvalidControlMessageNotificationConsumer) {
	require.Nil(m.t, m.consumer)
	m.consumer = c
}

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
		p2ptest.WithRole(flow.RoleConsensus),
		p2ptest.WithPeerScoringEnabled(idProvider),
		p2ptest.WithGossipSubRpcInspectorSuite(inspectorSuite1))

	node2, id2 := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
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
		inspectorSuite1.consumer.OnInvalidControlMessageNotification(&p2p.InvalidControlMessageNotification{
			PeerID:  node2.Host().ID(),
			MsgType: p2p.ControlMessageTypes()[rand.Intn(len(p2p.ControlMessageTypes()))],
			Count:   1,
			Err:     fmt.Errorf("invalid control message"),
		})
	}

	// checks no GossipSub message exchange should no longer happen between node1 and node2.
	p2pfixtures.EnsureNoPubsubExchangeBetweenGroups(t, ctx, []p2p.LibP2PNode{node1}, []p2p.LibP2PNode{node2}, func() (interface{}, channels.Topic) {
		blockTopic := channels.TopicFromChannel(channels.PushBlocks, sporkId)
		return unittest.ProposalFixture(), blockTopic
	})
}
