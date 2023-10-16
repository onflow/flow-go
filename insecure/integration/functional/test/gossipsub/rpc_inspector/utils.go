package rpc_inspector

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	mockery "github.com/stretchr/testify/mock"

	"github.com/onflow/flow-go/config"
	"github.com/onflow/flow-go/insecure/corruptlibp2p"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	mockp2p "github.com/onflow/flow-go/network/p2p/mock"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/network/p2p/tracer"
	"github.com/onflow/flow-go/utils/unittest"
)

// StartNodesAndEnsureConnected starts the victim and spammer node and ensures they are both connected.
func startNodesAndEnsureConnected(t *testing.T, ctx irrecoverable.SignalerContext, nodes []p2p.LibP2PNode, sporkID flow.Identifier) {
	p2ptest.StartNodes(t, ctx, nodes)
	// prior to the test we should ensure that spammer and victim connect.
	// this is vital as the spammer will circumvent the normal pubsub subscription mechanism and send iHAVE messages directly to the victim.
	// without a prior connection established, directly spamming pubsub messages may cause a race condition in the pubsub implementation.
	p2ptest.TryConnectionAndEnsureConnected(t, ctx, nodes)
	blockTopic := channels.TopicFromChannel(channels.PushBlocks, sporkID)
	p2ptest.EnsurePubsubMessageExchange(t, ctx, nodes, blockTopic, 1, func() interface{} {
		return unittest.ProposalFixture()
	})
}

func stopComponents(t *testing.T, cancel context.CancelFunc, nodes []p2p.LibP2PNode, components ...module.ReadyDoneAware) {
	p2ptest.StopNodes(t, nodes, cancel)
	unittest.RequireComponentsDoneBefore(t, time.Second, components...)
}

func randomClusterPrefixedTopic() channels.Topic {
	return channels.Topic(channels.SyncCluster(flow.ChainID(fmt.Sprintf("%d", rand.Uint64()))))
}

type onNotificationDissemination func(spammer *corruptlibp2p.GossipSubRouterSpammer) func(args mockery.Arguments)
type mockDistributorOption func(*mockp2p.GossipSubInspectorNotificationDistributor, *corruptlibp2p.GossipSubRouterSpammer)

func withExpectedNotificationDissemination(expectedNumOfTotalNotif int, f onNotificationDissemination) mockDistributorOption {
	return func(distributor *mockp2p.GossipSubInspectorNotificationDistributor, spammer *corruptlibp2p.GossipSubRouterSpammer) {
		distributor.
			On("Distribute", mockery.Anything).
			Times(expectedNumOfTotalNotif).
			Run(f(spammer)).
			Return(nil)
	}
}

// mockDistributorReadyDoneAware mocks the Ready and Done methods of the distributor to return a channel that is already closed,
// so that the distributor is considered ready and done when the test needs.
func mockDistributorReadyDoneAware(d *mockp2p.GossipSubInspectorNotificationDistributor) {
	d.On("Start", mockery.Anything).Return().Maybe()
	d.On("Ready").Return(func() <-chan struct{} {
		ch := make(chan struct{})
		close(ch)
		return ch
	}()).Maybe()
	d.On("Done").Return(func() <-chan struct{} {
		ch := make(chan struct{})
		close(ch)
		return ch
	}()).Maybe()
}

func meshTracerFixture(flowConfig *config.FlowConfig, idProvider module.IdentityProvider) *tracer.GossipSubMeshTracer {
	meshTracerCfg := &tracer.GossipSubMeshTracerConfig{
		Logger:                             unittest.Logger(),
		Metrics:                            metrics.NewNoopCollector(),
		IDProvider:                         idProvider,
		LoggerInterval:                     time.Second,
		HeroCacheMetricsFactory:            metrics.NewNoopHeroCacheMetricsFactory(),
		RpcSentTrackerCacheSize:            flowConfig.NetworkConfig.GossipSubConfig.RPCSentTrackerCacheSize,
		RpcSentTrackerWorkerQueueCacheSize: flowConfig.NetworkConfig.GossipSubConfig.RPCSentTrackerQueueCacheSize,
		RpcSentTrackerNumOfWorkers:         flowConfig.NetworkConfig.GossipSubConfig.RpcSentTrackerNumOfWorkers,
	}
	return tracer.NewGossipSubMeshTracer(meshTracerCfg)
}
