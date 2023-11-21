package tracer_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/config"
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

// TestGossipSubMeshTracer tests the GossipSub mesh tracer. It creates four nodes, one with a mesh tracer and three without.
// It then subscribes the nodes to the same topic and checks that the mesh tracer is able to detect the event of
// a node joining the mesh.
// It then checks that the mesh tracer is able to detect the event of a node leaving the mesh.
// One of the nodes is running with an unknown peer id, which the identity provider is mocked to return an error and
// the mesh tracer should log a warning message.
func TestGossipSubMeshTracer(t *testing.T) {
	defaultConfig, err := config.DefaultConfig()
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	sporkId := unittest.IdentifierFixture()
	idProvider := mockmodule.NewIdentityProvider(t)
	defer cancel()

	channel1 := channels.PushBlocks
	topic1 := channels.TopicFromChannel(channel1, sporkId)
	channel2 := channels.PushReceipts
	topic2 := channels.TopicFromChannel(channel2, sporkId)

	loggerCycle := atomic.NewInt32(0)
	warnLoggerCycle := atomic.NewInt32(0)

	// logger hook to count the number of times the meshTracer logs at the interval specified.
	hook := zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
		if level == zerolog.DebugLevel {
			if message == tracer.MeshLogIntervalMsg {
				loggerCycle.Inc()
			}
		}

		if level == zerolog.WarnLevel {
			if message == tracer.MeshLogIntervalWarnMsg {
				warnLoggerCycle.Inc()
			}
		}
	})
	logger := zerolog.New(os.Stdout).Level(zerolog.DebugLevel).Hook(hook)

	// creates one node with a gossipsub mesh meshTracer, and the other nodes without a gossipsub mesh meshTracer.
	// we only need one node with a meshTracer to test the meshTracer.
	// meshTracer logs at 1 second intervals for sake of testing.
	collector := mockmodule.NewGossipSubLocalMeshMetrics(t)
	meshTracerCfg := &tracer.GossipSubMeshTracerConfig{
		Logger:                             logger,
		Metrics:                            collector,
		IDProvider:                         idProvider,
		LoggerInterval:                     time.Second,
		HeroCacheMetricsFactory:            metrics.NewNoopHeroCacheMetricsFactory(),
		RpcSentTrackerCacheSize:            defaultConfig.NetworkConfig.GossipSubConfig.RPCSentTrackerCacheSize,
		RpcSentTrackerWorkerQueueCacheSize: defaultConfig.NetworkConfig.GossipSubConfig.RPCSentTrackerQueueCacheSize,
		RpcSentTrackerNumOfWorkers:         defaultConfig.NetworkConfig.GossipSubConfig.RpcSentTrackerNumOfWorkers,
	}
	meshTracer := tracer.NewGossipSubMeshTracer(meshTracerCfg)
	tracerNode, tracerId := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		idProvider,
		p2ptest.WithGossipSubTracer(meshTracer),
		p2ptest.WithRole(flow.RoleConsensus))

	idProvider.On("ByPeerID", tracerNode.ID()).Return(&tracerId, true).Maybe()

	otherNode1, otherId1 := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		idProvider,
		p2ptest.WithRole(flow.RoleConsensus))
	idProvider.On("ByPeerID", otherNode1.ID()).Return(&otherId1, true).Maybe()

	otherNode2, otherId2 := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		idProvider,
		p2ptest.WithRole(flow.RoleConsensus))
	idProvider.On("ByPeerID", otherNode2.ID()).Return(&otherId2, true).Maybe()

	// create a node that does not have a valid flow identity to test whether mesh tracer logs a warning.
	unknownNode, unknownId := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		idProvider,
		p2ptest.WithRole(flow.RoleConsensus))
	idProvider.On("ByPeerID", unknownNode.ID()).Return(nil, false).Maybe()

	nodes := []p2p.LibP2PNode{tracerNode, otherNode1, otherNode2, unknownNode}
	ids := flow.IdentityList{&tracerId, &otherId1, &otherId2, &unknownId}

	p2ptest.StartNodes(t, signalerCtx, nodes)
	defer p2ptest.StopNodes(t, nodes, cancel)

	p2ptest.LetNodesDiscoverEachOther(t, ctx, nodes, ids)

	// all nodes subscribe to topic1
	// for topic 1 expect the meshTracer to be notified of the local mesh size being 1, 2, and 3 (when unknownNode, otherNode1, and otherNode2 join the mesh).
	collector.On("OnLocalMeshSizeUpdated", topic1.String(), 1).Twice() // 1 for the first subscription, 1 for the first leave
	collector.On("OnLocalMeshSizeUpdated", topic1.String(), 2).Twice() // 1 for the second subscription, 1 for the second leave
	collector.On("OnLocalMeshSizeUpdated", topic1.String(), 3).Once()  // 3 for the third subscription.

	for _, node := range nodes {
		_, err := node.Subscribe(
			topic1,
			validator.TopicValidator(
				unittest.Logger(),
				unittest.AllowAllPeerFilter()))
		require.NoError(t, err)
	}

	// the tracerNode and otherNode1 subscribe to topic2
	// for topic 2 expect the meshTracer to be notified of the local mesh size being 1 (when otherNode1 join the mesh).
	collector.On("OnLocalMeshSizeUpdated", topic2.String(), 1).Once()

	for _, node := range []p2p.LibP2PNode{tracerNode, otherNode1} {
		_, err := node.Subscribe(
			topic2,
			validator.TopicValidator(
				unittest.Logger(),
				unittest.AllowAllPeerFilter()))
		require.NoError(t, err)
	}

	// eventually, the meshTracer should have the other nodes in its mesh.
	assert.Eventually(t, func() bool {
		topic1MeshSize := 0
		for _, peer := range meshTracer.GetLocalMeshPeers(topic1.String()) {
			if peer == otherNode1.ID() || peer == otherNode2.ID() {
				topic1MeshSize++
			}
		}

		topic2MeshSize := 0
		for _, peer := range meshTracer.GetLocalMeshPeers(topic2.String()) {
			if peer == otherNode1.ID() {
				topic2MeshSize++
			}
		}

		return topic1MeshSize == 2 && topic2MeshSize == 1
	}, 2*time.Second, 10*time.Millisecond)

	// eventually, we expect the meshTracer to log the mesh at least once.
	assert.Eventually(t, func() bool {
		return loggerCycle.Load() > 0 && warnLoggerCycle.Load() > 0
	}, 2*time.Second, 10*time.Millisecond)

	// expect the meshTracer to be notified of the local mesh size being (when all nodes leave the mesh).
	collector.On("OnLocalMeshSizeUpdated", topic1.String(), 0).Once()

	// all nodes except the tracerNode unsubscribe from the topic1, which triggers sending a PRUNE to the tracerNode for each unsubscription.
	// We expect the tracerNode to remove the otherNode1, otherNode2, and unknownNode from its mesh.
	require.NoError(t, otherNode1.Unsubscribe(topic1))
	require.NoError(t, otherNode2.Unsubscribe(topic1))
	require.NoError(t, unknownNode.Unsubscribe(topic1))

	assert.Eventually(t, func() bool {
		// eventually, the tracerNode should not have the other node in its mesh for topic1.
		for _, peer := range meshTracer.GetLocalMeshPeers(topic1.String()) {
			if peer == otherNode1.ID() || peer == otherNode2.ID() || peer == unknownNode.ID() {
				return false
			}
		}

		// but the tracerNode should still have the otherNode1 in its mesh for topic2.
		for _, peer := range meshTracer.GetLocalMeshPeers(topic2.String()) {
			if peer != otherNode1.ID() {
				return false
			}
		}
		return true
	}, 2*time.Second, 10*time.Millisecond)
}
