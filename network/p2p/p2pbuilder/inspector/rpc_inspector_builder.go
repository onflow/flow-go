package inspector

import (
	"github.com/rs/zerolog"

	netconf "github.com/onflow/flow-go/config/network"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/distributor"
	p2pconfig "github.com/onflow/flow-go/network/p2p/p2pbuilder/config"
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
	networkType      p2p.NetworkingType
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
		networkType:      p2p.PublicNetwork,
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
func (b *GossipSubInspectorBuilder) SetNetworkType(networkType p2p.NetworkingType) *GossipSubInspectorBuilder {
	b.networkType = networkType
	return b
}

// buildGossipSubMetricsInspector builds the gossipsub rpc metrics inspector.
func (b *GossipSubInspectorBuilder) buildGossipSubMetricsInspector() p2p.GossipSubRPCInspector {

	return metricsInspector
}

// buildGossipSubValidationInspector builds the gossipsub rpc validation inspector.
func (b *GossipSubInspectorBuilder) buildGossipSubValidationInspector() (p2p.GossipSubRPCInspector, *distributor.GossipSubInspectorNotifDistributor, error) {

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
	return , nil
}
