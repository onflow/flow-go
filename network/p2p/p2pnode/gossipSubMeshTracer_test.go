package p2pnode_test

import (
	"context"
	"testing"
	"time"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/p2p"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestGossipSubMeshTracer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	sporkId := unittest.IdentifierFixture()
	idProvider := mockmodule.NewIdentityProvider(t)
	defer cancel()

	// lg := unittest.Logger()

	// creates one node with a gossipsub mesh tracer, and the other node without a gossipsub mesh tracer.
	// we only need one node with a tracer to test the tracer.
	// tracer logs at 1 second intervals for sake of testing.
	// tracer := p2pnode.NewGossipSubMeshTracer(lg, metrics.NewNoopCollector(), idProvider, 1*time.Second)
	tracerNode, tracerId := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		// p2ptest.WithGossipSubTracer(tracer),
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
}
