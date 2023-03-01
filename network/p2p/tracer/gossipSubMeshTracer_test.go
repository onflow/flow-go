package tracer_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/network/p2p/tracer"
	validator "github.com/onflow/flow-go/network/validator/pubsub"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestGossipSubMeshTracer tests the gossipsub mesh tracer. It creates two nodes, one with a mesh tracer and one without.
// It then subscribes the nodes to the same topic and checks that the mesh tracer is able to detect the event of
// a node joining the mesh.
// It then checks that the mesh tracer is able to detect the event of a node leaving the mesh.
func TestGossipSubMeshTracer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	sporkId := unittest.IdentifierFixture()
	idProvider := mockmodule.NewIdentityProvider(t)
	defer cancel()

	loggerCycle := atomic.NewInt32(0)
	hook := zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
		fmt.Println(message)
		if level == zerolog.InfoLevel {
			if message == tracer.MeshLogIntervalMsg {
				loggerCycle.Inc()
			}
		}
	})
	logger := zerolog.New(os.Stdout).Level(zerolog.InfoLevel).Hook(hook)

	// creates one node with a gossipsub mesh meshTracer, and the other nodes without a gossipsub mesh meshTracer.
	// we only need one node with a meshTracer to test the meshTracer.
	// meshTracer logs at 1 second intervals for sake of testing.
	collector := mockmodule.NewGossipSubLocalMeshMetrics(t)
	meshTracer := tracer.NewGossipSubMeshTracer(logger, collector, idProvider, 1*time.Second)
	tracerNode, tracerId := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		p2ptest.WithGossipSubTracer(meshTracer),
		p2ptest.WithRole(flow.RoleConsensus))

	idProvider.On("ByPeerID", tracerNode.Host().ID()).Return(&tracerId, true).Maybe()

	otherNode1, otherId1 := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		p2ptest.WithRole(flow.RoleConsensus))
	idProvider.On("ByPeerID", otherNode1.Host().ID()).Return(&otherId1, true).Maybe()

	otherNode2, otherId2 := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		p2ptest.WithRole(flow.RoleConsensus))
	idProvider.On("ByPeerID", otherNode2.Host().ID()).Return(&otherId2, true).Maybe()

	nodes := []p2p.LibP2PNode{tracerNode, otherNode1, otherNode2}
	ids := flow.IdentityList{&tracerId, &otherId1, &otherId2}

	p2ptest.StartNodes(t, signalerCtx, nodes, 1*time.Second)
	defer p2ptest.StopNodes(t, nodes, cancel, 1*time.Second)

	p2ptest.LetNodesDiscoverEachOther(t, ctx, nodes, ids)

	topic := channels.TopicFromChannel(channels.PushBlocks, sporkId)

	// expect the meshTracer to be notified of the local mesh size being 1 and 2 (when otherNode1 and otherNode2 join the mesh).
	collector.On("OnLocalMeshSizeUpdated", topic.String(), 1).Twice() // 1 for the first subscription, 1 for the first leave
	collector.On("OnLocalMeshSizeUpdated", topic.String(), 2).Once()  // 2 for the second subscription.

	// all nodes subscribe to a legit topic
	_, err := tracerNode.Subscribe(
		topic,
		validator.TopicValidator(
			unittest.Logger(),
			unittest.AllowAllPeerFilter()))
	require.NoError(t, err)

	_, err = otherNode1.Subscribe(
		topic,
		validator.TopicValidator(
			unittest.Logger(),
			unittest.AllowAllPeerFilter()))
	require.NoError(t, err)

	_, err = otherNode2.Subscribe(
		topic,
		validator.TopicValidator(
			unittest.Logger(),
			unittest.AllowAllPeerFilter()))
	require.NoError(t, err)

	// eventually, the meshTracer should have the other nodes in its mesh.
	assert.Eventually(t, func() bool {
		count := 0
		for _, peer := range meshTracer.GetMeshPeers(topic.String()) {
			if peer == otherNode1.Host().ID() || peer == otherNode2.Host().ID() {
				count++
			}
		}
		return count == 2
	}, 2*time.Second, 10*time.Millisecond)

	// eventually, we expect the meshTracer to log the mesh at least once.
	assert.Eventually(t, func() bool {
		return loggerCycle.Load() > 0
	}, 2*time.Second, 10*time.Millisecond)

	// expect the meshTracer to be notified of the local mesh size being (when both nodes leave the mesh).
	collector.On("OnLocalMeshSizeUpdated", topic.String(), 0).Once()

	// otherNode1 unsubscribes from the topic which triggers sending a PRUNE to the tracerNode.
	// We expect the tracerNode to remove the otherNode1 and otherNode2 from its mesh.
	require.NoError(t, otherNode1.UnSubscribe(topic))
	require.NoError(t, otherNode2.UnSubscribe(topic))

	assert.Eventually(t, func() bool {
		// eventually, the meshTracer should not have the other node in its mesh.
		for _, peer := range meshTracer.GetMeshPeers(topic.String()) {
			if peer == otherNode1.Host().ID() || peer == otherNode2.Host().ID() {
				return false
			}
		}
		return true
	}, 2*time.Second, 10*time.Millisecond)
}
