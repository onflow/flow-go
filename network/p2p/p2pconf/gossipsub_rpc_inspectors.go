package p2pconf

// GossipSubRPCInspectorsConfig encompasses configuration related to gossipsub RPC message inspectors.
type GossipSubRPCInspectorsConfig struct {
	// GossipSubRPCValidationInspectorConfigs control message validation inspector validation configuration and limits.
	GossipSubRPCValidationInspectorConfigs `mapstructure:",squash"`
	// GossipSubRPCMetricsInspectorConfigs control message metrics inspector configuration.
	GossipSubRPCMetricsInspectorConfigs `mapstructure:",squash"`
	// GossipSubRPCInspectorNotificationCacheSize size of the queue for notifications about invalid RPC messages.
	GossipSubRPCInspectorNotificationCacheSize uint32 `mapstructure:"gossipsub-rpc-inspector-notification-cache-size"`
}

// GossipSubRPCValidationInspectorConfigs validation limits used for gossipsub RPC control message inspection.
type GossipSubRPCValidationInspectorConfigs struct {
	ClusterPrefixedMessageConfig `mapstructure:",squash"`
	IWantRPCInspectionConfig     `mapstructure:",squash"`
	// NumberOfWorkers number of worker pool workers.
	NumberOfWorkers int `validate:"gte=1" mapstructure:"gossipsub-rpc-validation-inspector-workers"`
	// CacheSize size of the queue used by worker pool for the control message validation inspector.
	CacheSize uint32 `validate:"gte=100" mapstructure:"gossipsub-rpc-validation-inspector-queue-cache-size"`
	// IHaveMaxSampleSize the max number of ihave messages in a sample to be inspected.
	IHaveMaxSampleSize int `validate:"gte=5000" mapstructure:"gossipsub-rpc-ihave-max-sample-size"`
	// ControlMessageMaxSampleSize the max sample size used for control message validation of GRAFT and PRUNE.
	ControlMessageMaxSampleSize int `validate:"gte=1000" mapstructure:"gossipsub-rpc-control-message-max-sample-size"`
}

// IWantRPCInspectionConfig validation configuration for iWANT RPC control messages.
type IWantRPCInspectionConfig struct {
	// MaxSampleSize max inspection sample size to use.
	MaxSampleSize uint `validate:"gt=0" mapstructure:"gossipsub-rpc-iwant-max-sample-size"`
	// CacheMissThreshold the threshold of missing corresponding iHave messages for iWant messages received before an invalid control message notification is disseminated.
	CacheMissThreshold float64 `validate:"gt=0" mapstructure:"gossipsub-rpc-iwant-cache-miss-threshold"`
}

// ClusterPrefixedMessageConfig configuration values for cluster prefixed control message validation.
type ClusterPrefixedMessageConfig struct {
	// ClusterPrefixHardThreshold the upper bound on the amount of cluster prefixed control messages that will be processed
	// before a node starts to get penalized. This allows LN nodes to process some cluster prefixed control messages during startup
	// when the cluster ID's provider is set asynchronously. It also allows processing of some stale messages that may be sent by nodes
	// that fall behind in the protocol. After the amount of cluster prefixed control messages processed exceeds this threshold the node
	// will be pushed to the edge of the network mesh.
	ClusterPrefixHardThreshold float64 `validate:"gt=0" mapstructure:"gossipsub-rpc-cluster-prefixed-hard-threshold"`
	// ClusterPrefixedControlMsgsReceivedCacheSize size of the cache used to track the amount of cluster prefixed topics received by peers.
	ClusterPrefixedControlMsgsReceivedCacheSize uint32 `validate:"gt=0" mapstructure:"gossipsub-cluster-prefix-tracker-cache-size"`
	// ClusterPrefixedControlMsgsReceivedCacheDecay decay val used for the geometric decay of cache counters used to keep track of cluster prefixed topics received by peers.
	ClusterPrefixedControlMsgsReceivedCacheDecay float64 `validate:"gt=0" mapstructure:"gossipsub-cluster-prefix-tracker-cache-decay"`
}

// GossipSubRPCMetricsInspectorConfigs rpc metrics observer inspector configuration.
type GossipSubRPCMetricsInspectorConfigs struct {
	// NumberOfWorkers number of worker pool workers.
	NumberOfWorkers int `validate:"gte=1" mapstructure:"gossipsub-rpc-metrics-inspector-workers"`
	// CacheSize size of the queue used by worker pool for the control message metrics inspector.
	CacheSize uint32 `validate:"gt=0" mapstructure:"gossipsub-rpc-metrics-inspector-cache-size"`
}
