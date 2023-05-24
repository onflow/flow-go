package inspector

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool/queue"
	"github.com/onflow/flow-go/module/metrics"
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
	inspectorsConfig *GossipSubRPCInspectorsConfig
	metricsCfg       *p2pconfig.MetricsConfig
	idProvider       module.IdentityProvider
	inspectorMetrics module.GossipSubRpcValidationInspectorMetrics
	publicNetwork    bool
}

// NewGossipSubInspectorBuilder returns new *GossipSubInspectorBuilder.
func NewGossipSubInspectorBuilder(logger zerolog.Logger, sporkID flow.Identifier, inspectorsConfig *GossipSubRPCInspectorsConfig, provider module.IdentityProvider, inspectorMetrics module.GossipSubRpcValidationInspectorMetrics) *GossipSubInspectorBuilder {
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
		publicNetwork:    p2p.PublicNetwork,
	}
}

// SetMetrics sets the network metrics and registry.
func (b *GossipSubInspectorBuilder) SetMetrics(metricsCfg *p2pconfig.MetricsConfig) *GossipSubInspectorBuilder {
	b.metricsCfg = metricsCfg
	return b
}

// SetPublicNetwork used to differentiate between libp2p nodes used for public vs private networks.
// Currently, there are different metrics collectors for public vs private networks.
func (b *GossipSubInspectorBuilder) SetPublicNetwork(public bool) *GossipSubInspectorBuilder {
	b.publicNetwork = public
	return b
}

// buildGossipSubMetricsInspector builds the gossipsub rpc metrics inspector.
func (b *GossipSubInspectorBuilder) buildGossipSubMetricsInspector() p2p.GossipSubRPCInspector {
	gossipSubMetrics := p2pnode.NewGossipSubControlMessageMetrics(b.metricsCfg.Metrics, b.logger)
	metricsInspector := inspector.NewControlMsgMetricsInspector(
		b.logger,
		gossipSubMetrics,
		b.inspectorsConfig.MetricsInspectorConfigs.NumberOfWorkers,
		[]queue.HeroStoreConfigOption{
			queue.WithHeroStoreSizeLimit(b.inspectorsConfig.MetricsInspectorConfigs.CacheSize),
			queue.WithHeroStoreCollector(metrics.GossipSubRPCMetricsObserverInspectorQueueMetricFactory(b.metricsCfg.HeroCacheFactory, b.publicNetwork)),
		}...)
	return metricsInspector
}

// validationInspectorConfig returns a new inspector.ControlMsgValidationInspectorConfig using configuration provided by the node builder.
func (b *GossipSubInspectorBuilder) validationInspectorConfig(validationConfigs *GossipSubRPCValidationInspectorConfigs) (*validation.ControlMsgValidationInspectorConfig, error) {
	// setup rpc validation configuration for each control message type
	graftValidationCfg, err := validation.NewCtrlMsgValidationConfig(p2p.CtrlMsgGraft, validationConfigs.GraftLimits)
	if err != nil {
		return nil, fmt.Errorf("failed to create gossupsub RPC validation configuration: %w", err)
	}
	pruneValidationCfg, err := validation.NewCtrlMsgValidationConfig(p2p.CtrlMsgPrune, validationConfigs.PruneLimits)
	if err != nil {
		return nil, fmt.Errorf("failed to create gossupsub RPC validation configuration: %w", err)
	}
	iHaveValidationCfg, err := validation.NewCtrlMsgValidationConfig(p2p.CtrlMsgIHave, validationConfigs.IHaveLimitsConfig.IHaveLimits, validationConfigs.IHaveLimitsConfig.IhaveConfigurationOpts()...)
	if err != nil {
		return nil, fmt.Errorf("failed to create gossupsub RPC validation configuration: %w", err)
	}
	// setup gossip sub RPC control message inspector config
	controlMsgRPCInspectorCfg := &validation.ControlMsgValidationInspectorConfig{
		ClusterPrefixedMessageConfig: &validation.ClusterPrefixedMessageConfig{
			ClusterPrefixHardThreshold:                   validationConfigs.ClusterPrefixHardThreshold,
			ClusterPrefixedControlMsgsReceivedCacheSize:  validationConfigs.ClusterPrefixedControlMsgsReceivedCacheSize,
			ClusterPrefixedControlMsgsReceivedCacheDecay: validationConfigs.ClusterPrefixedControlMsgsReceivedCacheDecay,
		},
		NumberOfWorkers: validationConfigs.NumberOfWorkers,
		InspectMsgStoreOpts: []queue.HeroStoreConfigOption{
			queue.WithHeroStoreSizeLimit(validationConfigs.CacheSize),
			queue.WithHeroStoreCollector(metrics.GossipSubRPCInspectorQueueMetricFactory(b.metricsCfg.HeroCacheFactory, b.publicNetwork))},
		GraftValidationCfg: graftValidationCfg,
		PruneValidationCfg: pruneValidationCfg,
		IHaveValidationCfg: iHaveValidationCfg,
	}
	return controlMsgRPCInspectorCfg, nil
}

// buildGossipSubValidationInspector builds the gossipsub rpc validation inspector.
func (b *GossipSubInspectorBuilder) buildGossipSubValidationInspector() (p2p.GossipSubRPCInspector, *distributor.GossipSubInspectorNotifDistributor, error) {
	controlMsgRPCInspectorCfg, err := b.validationInspectorConfig(b.inspectorsConfig.ValidationInspectorConfigs)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create gossipsub rpc inspector config: %w", err)
	}
	notificationDistributor := distributor.DefaultGossipSubInspectorNotificationDistributor(
		b.logger,
		[]queue.HeroStoreConfigOption{
			queue.WithHeroStoreSizeLimit(b.inspectorsConfig.GossipSubRPCInspectorNotificationCacheSize),
			queue.WithHeroStoreCollector(metrics.RpcInspectorNotificationQueueMetricFactory(b.metricsCfg.HeroCacheFactory, b.publicNetwork))}...)
	clusterPrefixedCacheCollector := metrics.GossipSubRPCInspectorClusterPrefixedCacheMetricFactory(b.metricsCfg.HeroCacheFactory, b.publicNetwork)
	rpcValidationInspector, err := validation.NewControlMsgValidationInspector(
		b.logger,
		b.sporkID,
		controlMsgRPCInspectorCfg,
		notificationDistributor,
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
