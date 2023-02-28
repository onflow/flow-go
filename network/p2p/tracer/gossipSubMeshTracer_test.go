package tracer_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/network/p2p/tracer"
	validator "github.com/onflow/flow-go/network/validator/pubsub"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestGossipSubMeshTracer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	sporkId := unittest.IdentifierFixture()
	idProvider := mockmodule.NewIdentityProvider(t)
	defer cancel()

	lg := unittest.Logger()

	// creates one node with a gossipsub mesh meshTracer, and the other node without a gossipsub mesh meshTracer.
	// we only need one node with a meshTracer to test the meshTracer.
	// meshTracer logs at 1 second intervals for sake of testing.
	meshTracer := tracer.NewGossipSubMeshTracer(lg, metrics.NewNoopCollector(), idProvider, 1*time.Second)
	tracerNode, tracerId := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		p2ptest.WithGossipSubTracer(meshTracer),
		p2ptest.WithRole(flow.RoleConsensus))

	idProvider.On("ByPeerID", tracerNode.Host().ID()).Return(&tracerId, true).Maybe()

	otherNode, otherId := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		p2ptest.WithRole(flow.RoleConsensus))
	idProvider.On("ByPeerID", otherNode.Host().ID()).Return(&otherId, true).Maybe()

	nodes := []p2p.LibP2PNode{tracerNode, otherNode}
	ids := flow.IdentityList{&tracerId, &otherId}

	p2ptest.StartNodes(t, signalerCtx, nodes, 1*time.Second)
	defer p2ptest.StopNodes(t, nodes, cancel, 1*time.Second)

	p2ptest.LetNodesDiscoverEachOther(t, ctx, nodes, ids)

	// both nodes subscribe to a legit topic
	topic := channels.TopicFromChannel(channels.PushBlocks, sporkId)
	_, err := tracerNode.Subscribe(
		topic,
		validator.TopicValidator(
			unittest.Logger(),
			unittest.AllowAllPeerFilter()))
	require.NoError(t, err)

	_, err = otherNode.Subscribe(
		topic,
		validator.TopicValidator(
			unittest.Logger(),
			unittest.AllowAllPeerFilter()))
	require.NoError(t, err)

	// eventually, the meshTracer should have the other node in its mesh.
	assert.Eventually(t, func() bool {
		for _, peer := range meshTracer.GetMeshPeers(topic.String()) {
			if peer == otherNode.Host().ID() {
				return true
			}
		}
		return false
	}, 2*time.Second, 10*time.Millisecond)

	// otherNode unsubscribes from the topic which triggers sending a PRUNE to the tracerNode.
	// We expect the tracerNode to remove the otherNode from its mesh.
	require.NoError(t, otherNode.UnSubscribe(topic))
	assert.Eventually(t, func() bool {
		// eventually, the meshTracer should not have the other node in its mesh.
		for _, peer := range meshTracer.GetMeshPeers(topic.String()) {
			if peer == otherNode.Host().ID() {
				return false
			}
		}
		return true
	}, 2*time.Second, 10*time.Millisecond)
}
