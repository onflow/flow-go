package p2pnode_test

import (
	"context"
	"fmt"
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
	"github.com/onflow/flow-go/network/p2p/p2pnode"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
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

	// creates one node with a gossipsub mesh tracer, and the other node without a gossipsub mesh tracer.
	// we only need one node with a tracer to test the tracer.
	// tracer logs at 1 second intervals for sake of testing.
	tracer := p2pnode.NewGossipSubMeshTracer(lg, metrics.NewNoopCollector(), idProvider, 1*time.Second)
	tracerNode, tracerId := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		p2ptest.WithGossipSubTracer(tracer),
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

	// eventually, the tracer should have the other node in its mesh.
	assert.Eventually(t, func() bool {
		if len(tracer.GetMeshPeers(topic.String())) != 0 {
			fmt.Println("mesh peers", tracer.GetMeshPeers(topic.String()))
		}
		for _, peer := range tracer.GetMeshPeers(topic.String()) {
			if peer == otherNode.Host().ID() {
				return true
			}
		}
		return false
	}, 5*time.Second, 10*time.Millisecond)
}
