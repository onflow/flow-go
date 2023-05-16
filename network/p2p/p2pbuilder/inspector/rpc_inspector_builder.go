package inspector

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
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
	networkType      p2p.NetworkType
}

// NewGossipSubInspectorBuilder returns new *GossipSubInspectorBuilder.
func NewGossipSubInspectorBuilder(logger zerolog.Logger, sporkID flow.Identifier, inspectorsConfig *GossipSubRPCInspectorsConfig) *GossipSubInspectorBuilder {
	return &GossipSubInspectorBuilder{
		logger:           logger,
		sporkID:          sporkID,
		inspectorsConfig: inspectorsConfig,
		metricsCfg: &p2pconfig.MetricsConfig{
			Metrics:          metrics.NewNoopCollector(),
			HeroCacheFactory: metrics.NewNoopHeroCacheMetricsFactory(),
		},
		networkType: p2p.PublicNetwork,
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
func (b *GossipSubInspectorBuilder) SetNetworkType(networkType p2p.NetworkType) *GossipSubInspectorBuilder {
	b.networkType = networkType
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
			queue.WithHeroStoreCollector(metrics.GossipSubRPCMetricsObserverInspectorQueueMetricFactory(b.metricsCfg.HeroCacheFactory, b.networkType)),
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
		NumberOfWorkers: validationConfigs.NumberOfWorkers,
		InspectMsgStoreOpts: []queue.HeroStoreConfigOption{
			queue.WithHeroStoreSizeLimit(validationConfigs.CacheSize),
			queue.WithHeroStoreCollector(metrics.GossipSubRPCInspectorQueueMetricFactory(b.metricsCfg.HeroCacheFactory, b.networkType))},
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
			queue.WithHeroStoreCollector(metrics.RpcInspectorNotificationQueueMetricFactory(b.metricsCfg.HeroCacheFactory, b.networkType))}...)

	rpcValidationInspector := validation.NewControlMsgValidationInspector(
		b.logger,
		b.sporkID,
		controlMsgRPCInspectorCfg,
		notificationDistributor,
		metrics.NewNoopCollector(),
	)
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
