package inspector

import (
	"fmt"

	"github.com/rs/zerolog"

	netconf "github.com/onflow/flow-go/config/network"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool/queue"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/distributor"
	"github.com/onflow/flow-go/network/p2p/inspector"
	"github.com/onflow/flow-go/network/p2p/inspector/validation"
	p2pconfig "github.com/onflow/flow-go/network/p2p/p2pbuilder/config"
	"github.com/onflow/flow-go/network/p2p/p2pbuilder/inspector/suite"
	"github.com/onflow/flow-go/network/p2p/p2pnode"
)

// GossipSubInspectorBuilder builder that constructs all rpc inspectors used by gossip sub. The following
// rpc inspectors are created with this builder.
// - validation inspector: performs validation on all control messages.
// - metrics inspector: observes metrics for each rpc message received.
type GossipSubInspectorBuilder struct {
	logger           zerolog.Logger
	sporkID          flow.Identifier
	inspectorsConfig *netconf.GossipSubRPCInspectorsConfig
	metricsCfg       *p2pconfig.MetricsConfig
	idProvider       module.IdentityProvider
	inspectorMetrics module.GossipSubRpcValidationInspectorMetrics
	networkType      network.NetworkingType
}

// NewGossipSubInspectorBuilder returns new *GossipSubInspectorBuilder.
func NewGossipSubInspectorBuilder(logger zerolog.Logger, sporkID flow.Identifier, inspectorsConfig *netconf.GossipSubRPCInspectorsConfig, provider module.IdentityProvider, inspectorMetrics module.GossipSubRpcValidationInspectorMetrics) *GossipSubInspectorBuilder {
	return &GossipSubInspectorBuilder{
		logger:           logger,
		sporkID:          sporkID,
		inspectorsConfig: inspectorsConfig,
		metricsCfg: &p2pconfig.MetricsConfig{
			Metrics:          metrics.NewNoopCollector(),
			HeroCacheFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		},
		idProvider:       provider,
		inspectorMetrics: inspectorMetrics,
		networkType:      network.PublicNetwork,
	}
}

// SetMetrics sets the network metrics and registry.
func (b *GossipSubInspectorBuilder) SetMetrics(metricsCfg *p2pconfig.MetricsConfig) *GossipSubInspectorBuilder {
	b.metricsCfg = metricsCfg
	return b
}

// SetNetworkType sets the network type for the inspector.
// This is used to determine if the node is running on a public or private network.
// Args:
// - networkType: the network type.
// Returns:
// - *GossipSubInspectorBuilder: the builder.
func (b *GossipSubInspectorBuilder) SetNetworkType(networkType network.NetworkingType) *GossipSubInspectorBuilder {
	b.networkType = networkType
	return b
}

// buildGossipSubMetricsInspector builds the gossipsub rpc metrics inspector.
func (b *GossipSubInspectorBuilder) buildGossipSubMetricsInspector() p2p.GossipSubRPCInspector {
	gossipSubMetrics := p2pnode.NewGossipSubControlMessageMetrics(b.metricsCfg.Metrics, b.logger)
	metricsInspector := inspector.NewControlMsgMetricsInspector(
		b.logger,
		gossipSubMetrics,
		b.inspectorsConfig.GossipSubRPCMetricsInspectorConfigs.NumberOfWorkers,
		[]queue.HeroStoreConfigOption{
			queue.WithHeroStoreSizeLimit(b.inspectorsConfig.GossipSubRPCMetricsInspectorConfigs.CacheSize),
			queue.WithHeroStoreCollector(metrics.GossipSubRPCMetricsObserverInspectorQueueMetricFactory(b.metricsCfg.HeroCacheFactory, b.networkType)),
		}...)
	return metricsInspector
}

// buildGossipSubValidationInspector builds the gossipsub rpc validation inspector.
func (b *GossipSubInspectorBuilder) buildGossipSubValidationInspector() (p2p.GossipSubRPCInspector, *distributor.GossipSubInspectorNotifDistributor, error) {
	notificationDistributor := distributor.DefaultGossipSubInspectorNotificationDistributor(
		b.logger,
		[]queue.HeroStoreConfigOption{
			queue.WithHeroStoreSizeLimit(b.inspectorsConfig.GossipSubRPCInspectorNotificationCacheSize),
			queue.WithHeroStoreCollector(metrics.RpcInspectorNotificationQueueMetricFactory(b.metricsCfg.HeroCacheFactory, b.networkType))}...)

	inspectMsgQueueCacheCollector := metrics.GossipSubRPCInspectorQueueMetricFactory(b.metricsCfg.HeroCacheFactory, b.networkType)
	clusterPrefixedCacheCollector := metrics.GossipSubRPCInspectorClusterPrefixedCacheMetricFactory(b.metricsCfg.HeroCacheFactory, b.networkType)
	rpcValidationInspector, err := validation.NewControlMsgValidationInspector(
		b.logger,
		b.sporkID,
		&b.inspectorsConfig.GossipSubRPCValidationInspectorConfigs,
		notificationDistributor,
		inspectMsgQueueCacheCollector,
		clusterPrefixedCacheCollector,
		b.idProvider,
		b.inspectorMetrics,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create new control message valiadation inspector: %w", err)
	}
	return rpcValidationInspector, notificationDistributor, nil
}

// Build builds the rpc inspectors used by gossipsub.
// Any returned error from this func indicates a problem setting up rpc inspectors.
// In libp2p node setup, the returned error should be treated as a fatal error.
func (b *GossipSubInspectorBuilder) Build() (p2p.GossipSubInspectorSuite, error) {
	metricsInspector := b.buildGossipSubMetricsInspector()
	validationInspector, notificationDistributor, err := b.buildGossipSubValidationInspector()
	if err != nil {
		return nil, err
	}
	return suite.NewGossipSubInspectorSuite([]p2p.GossipSubRPCInspector{metricsInspector, validationInspector}, notificationDistributor), nil
}
