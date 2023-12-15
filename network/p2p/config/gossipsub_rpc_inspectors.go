package p2pconfig

// RpcInspectorParameters keys.
const (
	ValidationConfigKey      = "validation"
	MetricsConfigKey         = "metrics"
	NotificationCacheSizeKey = "notification-cache-size"
)

// RpcInspectorParameters contains the "numerical values" for the gossipsub RPC control message inspectors parameters.
type RpcInspectorParameters struct {
	// RpcValidationInspector control message validation inspector validation configuration and limits.
	Validation RpcValidationInspector `mapstructure:"validation"`
	// RpcMetricsInspectorConfigs control message metrics inspector configuration.
	Metrics RpcMetricsInspectorConfigs `mapstructure:"metrics"`
	// NotificationCacheSize size of the queue for notifications about invalid RPC messages.
	NotificationCacheSize uint32 `mapstructure:"notification-cache-size"`
}

// RpcValidationInspectorParameters keys.
const (
	ClusterPrefixedMessageConfigKey   = "cluster-prefixed-messages"
	IWantConfigKey                    = "iwant"
	IHaveConfigKey                    = "ihave"
	QueueSizeKey                      = "queue-size"
	GraftPruneMessageMaxSampleSizeKey = "graft-and-prune-message-max-sample-size"
	MessageMaxSampleSizeKey           = "message-max-sample-size"
	MessageErrorThresholdKey          = "error-threshold"
)

// RpcValidationInspector validation limits used for gossipsub RPC control message inspection.
type RpcValidationInspector struct {
	ClusterPrefixedMessage ClusterPrefixedMessageInspectionParameters `mapstructure:"cluster-prefixed-messages"`
	IWant                  IWantRPCInspectionParameters               `mapstructure:"iwant"`
	IHave                  IHaveRpcInspectionParameters               `mapstructure:"ihave"`
	// NumberOfWorkers number of worker pool workers.
	NumberOfWorkers int `validate:"gte=1" mapstructure:"workers"`
	// QueueSize size of the queue used by worker pool for the control message validation inspector.
	QueueSize uint32 `validate:"gte=100" mapstructure:"queue-size"`
	// GraftPruneMessageMaxSampleSize the max sample size used for control message validation of GRAFT and PRUNE. If the total number of control messages (GRAFT or PRUNE)
	// exceeds this max sample size then the respective message will be truncated to this value before being processed.
	GraftPruneMessageMaxSampleSize int `validate:"gte=1000" mapstructure:"graft-and-prune-message-max-sample-size"`
	// RPCMessageMaxSampleSize the max sample size used for RPC message validation. If the total number of RPC messages exceeds this value a sample will be taken but messages will not be truncated.
	MessageMaxSampleSize int `validate:"gte=1000" mapstructure:"message-max-sample-size"`
	// RPCMessageErrorThreshold the threshold at which an error will be returned if the number of invalid RPC messages exceeds this value.
	MessageErrorThreshold int `validate:"gte=500" mapstructure:"error-threshold"`
}

const (
	MaxSampleSizeKey           = "max-sample-size"
	MaxMessageIDSampleSizeKey  = "max-message-id-sample-size"
	CacheMissThresholdKey      = "cache-miss-threshold"
	CacheMissCheckSizeKey      = "cache-miss-check-size"
	DuplicateMsgIDThresholdKey = "duplicate-message-id-threshold"
)

// IWantRPCInspectionParameters contains the "numerical values" for the iwant rpc control message inspection.
type IWantRPCInspectionParameters struct {
	// MaxSampleSize max inspection sample size to use. If the total number of iWant control messages
	// exceeds this max sample size then the respective message will be truncated before being processed.
	MaxSampleSize uint `validate:"gt=0" mapstructure:"max-sample-size"`
	// MaxMessageIDSampleSize max inspection sample size to use for iWant message ids. Each iWant message includes a list of message ids
	// each, if the size of this list exceeds the configured max message id sample size the list of message ids will be truncated.
	MaxMessageIDSampleSize int `validate:"gte=1000" mapstructure:"max-message-id-sample-size"`
	// CacheMissThreshold the threshold of missing corresponding iHave messages for iWant messages received before an invalid control message notification is disseminated.
	// If the cache miss threshold is exceeded an invalid control message notification is disseminated and the sender will be penalized.
	CacheMissThreshold float64 `validate:"gt=0" mapstructure:"cache-miss-threshold"`
	// CacheMissCheckSize the iWants size at which message id cache misses will be checked.
	CacheMissCheckSize int `validate:"gt=0" mapstructure:"cache-miss-check-size"`
	// DuplicateMsgIDThreshold maximum allowed duplicate message IDs in a single iWant control message.
	// If the duplicate message threshold is exceeded an invalid control message notification is disseminated and the sender will be penalized.
	DuplicateMsgIDThreshold float64 `validate:"gt=0" mapstructure:"duplicate-message-id-threshold"`
}

// IHaveRpcInspectionParameters contains the "numerical values" for ihave rpc control inspection.
type IHaveRpcInspectionParameters struct {
	// MaxSampleSize max inspection sample size to use. If the number of ihave messages exceeds this configured value
	// the control message ihaves will be truncated to the max sample size. This sample is randomly selected.
	MaxSampleSize int `validate:"gte=1000" mapstructure:"max-sample-size"`
	// MaxMessageIDSampleSize max inspection sample size to use for iHave message ids. Each ihave message includes a list of message ids
	// each, if the size of this list exceeds the configured max message id sample size the list of message ids will be truncated.
	MaxMessageIDSampleSize int `validate:"gte=1000" mapstructure:"max-message-id-sample-size"`
}

const (
	HardThresholdKey     = "hard-threshold"
	TrackerCacheSizeKey  = "tracker-cache-size"
	TrackerCacheDecayKey = "tracker-cache-decay"
)

// ClusterPrefixedMessageInspectionParameters contains the "numerical values" for cluster prefixed control message inspection.
type ClusterPrefixedMessageInspectionParameters struct {
	// HardThreshold the upper bound on the amount of cluster prefixed control messages that will be processed
	// before a node starts to get penalized. This allows LN nodes to process some cluster prefixed control messages during startup
	// when the cluster ID's provider is set asynchronously. It also allows processing of some stale messages that may be sent by nodes
	// that fall behind in the protocol. After the amount of cluster prefixed control messages processed exceeds this threshold the node
	// will be pushed to the edge of the network mesh.
	HardThreshold float64 `validate:"gte=0" mapstructure:"hard-threshold"`
	// ControlMsgsReceivedCacheSize size of the cache used to track the amount of cluster prefixed topics received by peers.
	ControlMsgsReceivedCacheSize uint32 `validate:"gt=0" mapstructure:"tracker-cache-size"`
	// ControlMsgsReceivedCacheDecay decay val used for the geometric decay of cache counters used to keep track of cluster prefixed topics received by peers.
	ControlMsgsReceivedCacheDecay float64 `validate:"gt=0" mapstructure:"tracker-cache-decay"`
}

const (
	NumberOfWorkersKey = "workers"
)

// RpcMetricsInspectorConfigs contains the "numerical values" for the gossipsub RPC control message metrics inspectors parameters.
type RpcMetricsInspectorConfigs struct {
	// NumberOfWorkers number of worker pool workers.
	NumberOfWorkers int `validate:"gte=1" mapstructure:"workers"`
	// CacheSize size of the queue used by worker pool for the control message metrics inspector.
	CacheSize uint32 `validate:"gt=0" mapstructure:"cache-size"`
}
