package inspector

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool/queue"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/distributor"
	"github.com/onflow/flow-go/network/p2p/inspector"
	"github.com/onflow/flow-go/network/p2p/inspector/validation"
	"github.com/onflow/flow-go/network/p2p/p2pnode"
)

type metricsCollectorFactory func() *metrics.HeroCacheCollector

// GossipSubRPCValidationInspectorConfigs validation limits used for gossipsub RPC control message inspection.
type GossipSubRPCValidationInspectorConfigs struct {
	// NumberOfWorkers number of worker pool workers.
	NumberOfWorkers int
	// CacheSize size of the queue used by worker pool for the control message validation inspector.
	CacheSize uint32
	// GraftLimits GRAFT control message validation limits.
	GraftLimits map[string]int
	// PruneLimits PRUNE control message validation limits.
	PruneLimits map[string]int
	// ClusterPrefixDiscardThreshold the upper bound on the amount of cluster prefixed control messages that will be processed
	// before a node starts to get penalized.
	ClusterPrefixDiscardThreshold uint64
}

// GossipSubRPCMetricsInspectorConfigs rpc metrics observer inspector configuration.
type GossipSubRPCMetricsInspectorConfigs struct {
	// NumberOfWorkers number of worker pool workers.
	NumberOfWorkers int
	// CacheSize size of the queue used by worker pool for the control message metrics inspector.
	CacheSize uint32
}

// GossipSubRPCInspectorsConfig encompasses configuration related to gossipsub RPC message inspectors.
type GossipSubRPCInspectorsConfig struct {
	// GossipSubRPCInspectorNotificationCacheSize size of the queue for notifications about invalid RPC messages.
	GossipSubRPCInspectorNotificationCacheSize uint32
	// ValidationInspectorConfigs control message validation inspector validation configuration and limits.
	ValidationInspectorConfigs *GossipSubRPCValidationInspectorConfigs
	// MetricsInspectorConfigs control message metrics inspector configuration.
	MetricsInspectorConfigs *GossipSubRPCMetricsInspectorConfigs
}

func DefaultGossipSubRPCInspectorsConfig() *GossipSubRPCInspectorsConfig {
	return &GossipSubRPCInspectorsConfig{
		GossipSubRPCInspectorNotificationCacheSize: distributor.DefaultGossipSubInspectorNotificationQueueCacheSize,
		ValidationInspectorConfigs: &GossipSubRPCValidationInspectorConfigs{
			NumberOfWorkers: validation.DefaultNumberOfWorkers,
			CacheSize:       validation.DefaultControlMsgValidationInspectorQueueCacheSize,
			GraftLimits: map[string]int{
				validation.DiscardThresholdMapKey: validation.DefaultGraftDiscardThreshold,
				validation.SafetyThresholdMapKey:  validation.DefaultGraftSafetyThreshold,
				validation.RateLimitMapKey:        validation.DefaultGraftRateLimit,
			},
			PruneLimits: map[string]int{
				validation.DiscardThresholdMapKey: validation.DefaultPruneDiscardThreshold,
				validation.SafetyThresholdMapKey:  validation.DefaultPruneSafetyThreshold,
				validation.RateLimitMapKey:        validation.DefaultPruneRateLimit,
			},
			ClusterPrefixDiscardThreshold: validation.DefaultClusterPrefixDiscardThreshold,
		},
		MetricsInspectorConfigs: &GossipSubRPCMetricsInspectorConfigs{
			NumberOfWorkers: inspector.DefaultControlMsgMetricsInspectorNumberOfWorkers,
			CacheSize:       inspector.DefaultControlMsgMetricsInspectorQueueCacheSize,
		},
	}
}

// GossipSubInspectorBuilder builder that constructs all rpc inspectors used by gossip sub. The following
// rpc inspectors are created with this builder.
// - validation inspector: performs validation on all control messages.
// - metrics inspector: observes metrics for each rpc message received.
type GossipSubInspectorBuilder struct {
	logger           zerolog.Logger
	sporkID          flow.Identifier
	inspectorsConfig *GossipSubRPCInspectorsConfig
	distributor      p2p.GossipSubInspectorNotificationDistributor
	netMetrics       module.NetworkMetrics
	metricsRegistry  prometheus.Registerer
	metricsEnabled   bool
	publicNetwork    bool
}

// NewGossipSubInspectorBuilder returns new *GossipSubInspectorBuilder.
func NewGossipSubInspectorBuilder(logger zerolog.Logger, sporkID flow.Identifier, inspectorsConfig *GossipSubRPCInspectorsConfig, distributor p2p.GossipSubInspectorNotificationDistributor) *GossipSubInspectorBuilder {
	return &GossipSubInspectorBuilder{
		logger:           logger,
		sporkID:          sporkID,
		inspectorsConfig: inspectorsConfig,
		distributor:      distributor,
		netMetrics:       metrics.NewNoopCollector(),
		metricsEnabled:   p2p.MetricsDisabled,
		publicNetwork:    p2p.PublicNetworkEnabled,
	}
}

// SetMetricsEnabled disable and enable metrics collection for the inspectors underlying hero store cache.
func (b *GossipSubInspectorBuilder) SetMetricsEnabled(metricsEnabled bool) *GossipSubInspectorBuilder {
	b.metricsEnabled = metricsEnabled
	return b
}

// SetMetrics sets the network metrics and registry.
func (b *GossipSubInspectorBuilder) SetMetrics(netMetrics module.NetworkMetrics, metricsRegistry prometheus.Registerer) *GossipSubInspectorBuilder {
	b.netMetrics = netMetrics
	b.metricsRegistry = metricsRegistry
	return b
}

// SetPublicNetwork used to differentiate between libp2p nodes used for public vs private networks.
// Currently, there are different metrics collectors for public vs private networks.
func (b *GossipSubInspectorBuilder) SetPublicNetwork(public bool) *GossipSubInspectorBuilder {
	b.publicNetwork = public
	return b
}

// heroStoreOpts builds the gossipsub rpc validation inspector hero store opts.
// These options are used in the underlying worker pool hero store.
func (b *GossipSubInspectorBuilder) heroStoreOpts(size uint32, collectorFactory metricsCollectorFactory) []queue.HeroStoreConfigOption {
	heroStoreOpts := []queue.HeroStoreConfigOption{queue.WithHeroStoreSizeLimit(size)}
	if b.metricsEnabled {
		heroStoreOpts = append(heroStoreOpts, queue.WithHeroStoreCollector(collectorFactory()))
	}
	return heroStoreOpts
}

func (b *GossipSubInspectorBuilder) validationInspectorMetricsCollectorFactory() metricsCollectorFactory {
	return func() *metrics.HeroCacheCollector {
		return metrics.GossipSubRPCValidationInspectorQueueMetricFactory(b.publicNetwork, b.metricsRegistry)
	}
}

func (b *GossipSubInspectorBuilder) metricsInspectorMetricsCollectorFactory() metricsCollectorFactory {
	return func() *metrics.HeroCacheCollector {
		return metrics.GossipSubRPCMetricsObserverInspectorQueueMetricFactory(b.publicNetwork, b.metricsRegistry)
	}
}

// buildGossipSubMetricsInspector builds the gossipsub rpc metrics inspector.
func (b *GossipSubInspectorBuilder) buildGossipSubMetricsInspector() p2p.GossipSubRPCInspector {
	gossipSubMetrics := p2pnode.NewGossipSubControlMessageMetrics(b.netMetrics, b.logger)
	metricsInspectorHeroStoreOpts := b.heroStoreOpts(b.inspectorsConfig.MetricsInspectorConfigs.CacheSize, b.metricsInspectorMetricsCollectorFactory())
	metricsInspector := inspector.NewControlMsgMetricsInspector(b.logger, gossipSubMetrics, b.inspectorsConfig.MetricsInspectorConfigs.NumberOfWorkers, metricsInspectorHeroStoreOpts...)
	return metricsInspector
}

// validationInspectorConfig returns a new inspector.ControlMsgValidationInspectorConfig using configuration provided by the node builder.
func (b *GossipSubInspectorBuilder) validationInspectorConfig(validationConfigs *GossipSubRPCValidationInspectorConfigs, opts ...queue.HeroStoreConfigOption) (*validation.ControlMsgValidationInspectorConfig, error) {
	// setup rpc validation configuration for each control message type
	graftValidationCfg, err := validation.NewCtrlMsgValidationConfig(p2p.CtrlMsgGraft, validationConfigs.GraftLimits)
	if err != nil {
		return nil, fmt.Errorf("failed to create gossupsub RPC validation configuration: %w", err)
	}
	pruneValidationCfg, err := validation.NewCtrlMsgValidationConfig(p2p.CtrlMsgPrune, validationConfigs.PruneLimits)
	if err != nil {
		return nil, fmt.Errorf("failed to create gossupsub RPC validation configuration: %w", err)
	}

	// setup gossip sub RPC control message inspector config
	controlMsgRPCInspectorCfg := &validation.ControlMsgValidationInspectorConfig{
		NumberOfWorkers:               validationConfigs.NumberOfWorkers,
		InspectMsgStoreOpts:           opts,
		GraftValidationCfg:            graftValidationCfg,
		PruneValidationCfg:            pruneValidationCfg,
		ClusterPrefixDiscardThreshold: validationConfigs.ClusterPrefixDiscardThreshold,
	}
	return controlMsgRPCInspectorCfg, nil
}

// buildGossipSubValidationInspector builds the gossipsub rpc validation inspector.
func (b *GossipSubInspectorBuilder) buildGossipSubValidationInspector() (p2p.GossipSubRPCInspector, error) {
	rpcValidationInspectorHeroStoreOpts := b.heroStoreOpts(b.inspectorsConfig.ValidationInspectorConfigs.CacheSize, b.validationInspectorMetricsCollectorFactory())
	controlMsgRPCInspectorCfg, err := b.validationInspectorConfig(b.inspectorsConfig.ValidationInspectorConfigs, rpcValidationInspectorHeroStoreOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create gossipsub rpc inspector config: %w", err)
	}
	rpcValidationInspector := validation.NewControlMsgValidationInspector(b.logger, b.sporkID, controlMsgRPCInspectorCfg, b.distributor)
	return rpcValidationInspector, nil
}

// Build builds the rpc inspectors used by gossipsub.
// Any returned error from this func indicates a problem setting up rpc inspectors.
// In libp2p node setup, the returned error should be treated as a fatal error.
func (b *GossipSubInspectorBuilder) Build() ([]p2p.GossipSubRPCInspector, error) {
	metricsInspector := b.buildGossipSubMetricsInspector()
	validationInspector, err := b.buildGossipSubValidationInspector()
	if err != nil {
		return nil, err
	}
	return []p2p.GossipSubRPCInspector{metricsInspector, validationInspector}, nil
}

// DefaultRPCValidationConfig returns default RPC control message inspector config.
func DefaultRPCValidationConfig(opts ...queue.HeroStoreConfigOption) *validation.ControlMsgValidationInspectorConfig {
	graftCfg, _ := validation.NewCtrlMsgValidationConfig(p2p.CtrlMsgGraft, validation.CtrlMsgValidationLimits{
		validation.DiscardThresholdMapKey: validation.DefaultGraftDiscardThreshold,
		validation.SafetyThresholdMapKey:  validation.DefaultGraftSafetyThreshold,
		validation.RateLimitMapKey:        validation.DefaultGraftRateLimit,
	})
	pruneCfg, _ := validation.NewCtrlMsgValidationConfig(p2p.CtrlMsgPrune, validation.CtrlMsgValidationLimits{
		validation.DiscardThresholdMapKey: validation.DefaultPruneDiscardThreshold,
		validation.SafetyThresholdMapKey:  validation.DefaultPruneSafetyThreshold,
		validation.RateLimitMapKey:        validation.DefaultPruneRateLimit,
	})

	return &validation.ControlMsgValidationInspectorConfig{
		NumberOfWorkers:               validation.DefaultNumberOfWorkers,
		InspectMsgStoreOpts:           opts,
		GraftValidationCfg:            graftCfg,
		PruneValidationCfg:            pruneCfg,
		ClusterPrefixDiscardThreshold: validation.DefaultClusterPrefixDiscardThreshold,
	}
}
