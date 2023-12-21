package p2pconf

// GossipSubRPCInspectorsConfig encompasses configuration related to gossipsub RPC message inspectors.
type GossipSubRPCInspectorsConfig struct {
	// GossipSubRPCValidationInspectorConfigs control message validation inspector validation configuration and limits.
	GossipSubRPCValidationInspectorConfigs `mapstructure:",squash"`
	// GossipSubRPCInspectorNotificationCacheSize size of the queue for notifications about invalid RPC messages.
	GossipSubRPCInspectorNotificationCacheSize uint32 `mapstructure:"gossipsub-rpc-inspector-notification-cache-size"`
}

// GossipSubRPCValidationInspectorConfigs validation limits used for gossipsub RPC control message inspection.
type GossipSubRPCValidationInspectorConfigs struct {
	ClusterPrefixedMessageConfig `mapstructure:",squash"`
	IWantRPCInspectionConfig     `mapstructure:",squash"`
	IHaveRPCInspectionConfig     `mapstructure:",squash"`
	// NumberOfWorkers number of worker pool workers.
	NumberOfWorkers int `validate:"gte=1" mapstructure:"gossipsub-rpc-validation-inspector-workers"`
	// CacheSize size of the queue used by worker pool for the control message validation inspector.
	CacheSize uint32 `validate:"gte=100" mapstructure:"gossipsub-rpc-validation-inspector-queue-cache-size"`
	// GraftPruneMessageMaxSampleSize the max sample size used for control message validation of GRAFT and PRUNE. If the total number of control messages (GRAFT or PRUNE)
	// exceeds this max sample size then the respective message will be truncated to this value before being processed.
	GraftPruneMessageMaxSampleSize int `validate:"gte=1000" mapstructure:"gossipsub-rpc-graft-and-prune-message-max-sample-size"`
}

// IWantRPCInspectionConfig validation configuration for iWANT RPC control messages.
type IWantRPCInspectionConfig struct {
	// MaxSampleSize max inspection sample size to use. If the total number of iWant control messages
	// exceeds this max sample size then the respective message will be truncated before being processed.
	MaxSampleSize uint `validate:"gt=0" mapstructure:"gossipsub-rpc-iwant-max-sample-size"`
	// MaxMessageIDSampleSize max inspection sample size to use for iWant message ids. Each iWant message includes a list of message ids
	// each, if the size of this list exceeds the configured max message id sample size the list of message ids will be truncated.
	MaxMessageIDSampleSize int `validate:"gte=1000" mapstructure:"gossipsub-rpc-iwant-max-message-id-sample-size"`
	// CacheMissThreshold the threshold of missing corresponding iHave messages for iWant messages received before an invalid control message notification is disseminated.
	// If the cache miss threshold is exceeded an invalid control message notification is disseminated and the sender will be penalized.
	CacheMissThreshold float64 `validate:"gt=0" mapstructure:"gossipsub-rpc-iwant-cache-miss-threshold"`
	// CacheMissCheckSize the iWants size at which message id cache misses will be checked.
	CacheMissCheckSize int `validate:"gte=1000" mapstructure:"gossipsub-rpc-iwant-cache-miss-check-size"`
	// DuplicateMsgIDThreshold maximum allowed duplicate message IDs in a single iWant control message.
	// If the duplicate message threshold is exceeded an invalid control message notification is disseminated and the sender will be penalized.
	DuplicateMsgIDThreshold float64 `validate:"gt=0" mapstructure:"gossipsub-rpc-iwant-duplicate-message-id-threshold"`
}

// IHaveRPCInspectionConfig validation configuration for iHave RPC control messages.
type IHaveRPCInspectionConfig struct {
	// MaxSampleSize max inspection sample size to use. If the number of ihave messages exceeds this configured value
	// the control message ihaves will be truncated to the max sample size. This sample is randomly selected.
	MaxSampleSize int `validate:"gte=1000" mapstructure:"gossipsub-rpc-ihave-max-sample-size"`
	// MaxMessageIDSampleSize max inspection sample size to use for iHave message ids. Each ihave message includes a list of message ids
	// each, if the size of this list exceeds the configured max message id sample size the list of message ids will be truncated.
	MaxMessageIDSampleSize int `validate:"gte=1000" mapstructure:"gossipsub-rpc-ihave-max-message-id-sample-size"`
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
