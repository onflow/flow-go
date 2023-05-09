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
	"github.com/onflow/flow-go/network/p2p/inspector/cache"
	"github.com/onflow/flow-go/network/p2p/inspector/validation"
	p2pconfig "github.com/onflow/flow-go/network/p2p/p2pbuilder/config"
	"github.com/onflow/flow-go/network/p2p/p2pbuilder/inspector/suite"
	"github.com/onflow/flow-go/network/p2p/p2pnode"
)

// GossipSubRPCValidationInspectorConfigs validation limits used for gossipsub RPC control message inspection.
type GossipSubRPCValidationInspectorConfigs struct {
	// NumberOfWorkers number of worker pool workers.
	NumberOfWorkers int
	// InspectMessageQueueCacheSize size of the queue used by worker pool for the control message validation inspector.
	InspectMessageQueueCacheSize uint32
	// ClusterPrefixedTopicsReceivedCacheSize size of the cache used to track the amount of cluster prefixed topics received by peers.
	ClusterPrefixedTopicsReceivedCacheSize uint32
	// GraftLimits GRAFT control message validation limits.
	GraftLimits map[string]int
	// PruneLimits PRUNE control message validation limits.
	PruneLimits map[string]int
	// ClusterPrefixDiscardThreshold the upper bound on the amount of cluster prefixed control messages that will be processed
	// before a node starts to get penalized.
	ClusterPrefixDiscardThreshold int64
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
			NumberOfWorkers:                        validation.DefaultNumberOfWorkers,
			InspectMessageQueueCacheSize:           validation.DefaultControlMsgValidationInspectorQueueCacheSize,
			ClusterPrefixedTopicsReceivedCacheSize: validation.DefaultClusterPrefixedTopicsReceivedCacheSize,
			ClusterPrefixDiscardThreshold:          validation.DefaultClusterPrefixDiscardThreshold,
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
	metricsCfg       *p2pconfig.MetricsConfig
	publicNetwork    bool
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
		publicNetwork: p2p.PublicNetwork,
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

	// setup gossip sub RPC control message inspector config
	controlMsgRPCInspectorCfg := &validation.ControlMsgValidationInspectorConfig{
		ClusterPrefixHardThreshold:             validationConfigs.ClusterPrefixDiscardThreshold,
		ClusterPrefixedTopicsReceivedCacheSize: validationConfigs.ClusterPrefixedTopicsReceivedCacheSize,
		NumberOfWorkers:                        validationConfigs.NumberOfWorkers,
		InspectMsgStoreOpts: []queue.HeroStoreConfigOption{
			queue.WithHeroStoreSizeLimit(validationConfigs.InspectMessageQueueCacheSize),
			queue.WithHeroStoreCollector(metrics.GossipSubRPCInspectorQueueMetricFactory(b.metricsCfg.HeroCacheFactory, b.publicNetwork))},
		GraftValidationCfg: graftValidationCfg,
		PruneValidationCfg: pruneValidationCfg,
	}
	return controlMsgRPCInspectorCfg, nil
}

// buildGossipSubValidationInspector builds the gossipsub rpc validation inspector.
func (b *GossipSubInspectorBuilder) buildGossipSubValidationInspector() (p2p.GossipSubRPCInspector, *distributor.GossipSubInspectorNotifDistributor, error) {
	controlMsgRPCInspectorCfg, err := b.validationInspectorConfig(b.inspectorsConfig.ValidationInspectorConfigs)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create gossipsub rpc inspector config: %w", err)
	}
	trackerOpts := make([]cache.RecordCacheConfigOpt, 0)
	if b.metricsEnabled {
		trackerOpts = append(trackerOpts, cache.WithMetricsCollector(metrics.GossipSubRPCInspectorClusterPrefixedCacheMetricFactory(b.metricsCfg.HeroCacheFactory, b.publicNetwork)))
	}

	notificationDistributor := distributor.DefaultGossipSubInspectorNotificationDistributor(
		b.logger,
		[]queue.HeroStoreConfigOption{
			queue.WithHeroStoreSizeLimit(b.inspectorsConfig.GossipSubRPCInspectorNotificationCacheSize),
			queue.WithHeroStoreCollector(metrics.GossipSubRPCInspectorQueueMetricFactory(b.metricsCfg.HeroCacheFactory, b.publicNetwork))}...)

	rpcValidationInspector := validation.NewControlMsgValidationInspector(
		b.logger,
		b.sporkID,
		controlMsgRPCInspectorCfg,
		notificationDistributor,
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
		NumberOfWorkers:                        validation.DefaultNumberOfWorkers,
		InspectMsgStoreOpts:                    opts,
		GraftValidationCfg:                     graftCfg,
		PruneValidationCfg:                     pruneCfg,
		ClusterPrefixHardThreshold:             validation.DefaultClusterPrefixDiscardThreshold,
		ClusterPrefixedTopicsReceivedCacheSize: validation.DefaultClusterPrefixedTopicsReceivedCacheSize,
	}
}
