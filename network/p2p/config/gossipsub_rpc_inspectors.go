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
	// NotificationCacheSize size of the queue for notifications about invalid RPC messages.
	NotificationCacheSize uint32 `mapstructure:"notification-cache-size"`
}

// RpcValidationInspectorParameters keys.
const (
	ClusterPrefixedMessageConfigKey = "cluster-prefixed-messages"
	IWantConfigKey                  = "iwant"
	IHaveConfigKey                  = "ihave"
	GraftPruneKey                   = "graft-and-prune"
	PublishMessagesConfigKey        = "publish-messages"
	InspectionQueueConfigKey        = "inspection-queue"
)

// RpcValidationInspector validation limits used for gossipsub RPC control message inspection.
type RpcValidationInspector struct {
	ClusterPrefixedMessage ClusterPrefixedMessageInspectionParameters `mapstructure:"cluster-prefixed-messages"`
	IWant                  IWantRpcInspectionParameters               `mapstructure:"iwant"`
	IHave                  IHaveRpcInspectionParameters               `mapstructure:"ihave"`
	GraftPrune             GraftPruneRpcInspectionParameters          `mapstructure:"graft-and-prune"`
	PublishMessages        PublishMessageInspectionParameters         `mapstructure:"publish-messages"`
	InspectionQueue        InspectionQueueParameters                  `mapstructure:"inspection-queue"`
}

const (
	QueueSizeKey = "queue-size"
)

type InspectionQueueParameters struct {
	// NumberOfWorkers number of worker pool workers.
	NumberOfWorkers int `validate:"gte=1" mapstructure:"workers"`
	// Size size of the queue used by worker pool for the control message validation inspector.
	Size uint32 `validate:"gte=100" mapstructure:"queue-size"`
}

const (
	MessageErrorThresholdKey = "error-threshold"
)

type PublishMessageInspectionParameters struct {
	// MaxSampleSize the max sample size used for RPC message validation. If the total number of RPC messages exceeds this value a sample will be taken but messages will not be truncated.
	MaxSampleSize int `validate:"gte=1000" mapstructure:"max-sample-size"`
	// ErrorThreshold the threshold at which an error will be returned if the number of invalid RPC messages exceeds this value.
	ErrorThreshold int `validate:"gte=500" mapstructure:"error-threshold"`
}

type GraftPruneRpcInspectionParameters struct {
	// MaxSampleSize the max sample size used for control message validation of GRAFT and PRUNE. If the total number of control messages (GRAFT or PRUNE)
	// exceeds this max sample size then the respective message will be truncated to this value before being processed.
	MaxSampleSize int `validate:"gte=1000" mapstructure:"max-sample-size"`

	// MaxTotalDuplicateTopicIdThreshold is the tolerance threshold for having duplicate topics in a single GRAFT or PRUNE message under inspection.
	// Ideally, a GRAFT or PRUNE message should not have any duplicate topics, hence a topic ID is counted as a duplicate only if it is repeated more than once.
	// When the total number of duplicate topic ids in a single GRAFT or PRUNE message exceeds this threshold, the inspection of message will fail.
	MaxTotalDuplicateTopicIdThreshold int `validate:"gte=0" mapstructure:"max-total-duplicate-topic-id-threshold"`
}

const (
	MaxSampleSizeKey           = "max-sample-size"
	MaxMessageIDSampleSizeKey  = "max-message-id-sample-size"
	CacheMissThresholdKey      = "cache-miss-threshold"
	CacheMissCheckSizeKey      = "cache-miss-check-size"
	DuplicateMsgIDThresholdKey = "duplicate-message-id-threshold"
)

// IWantRpcInspectionParameters contains the "numerical values" for the iwant rpc control message inspection.
type IWantRpcInspectionParameters struct {
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

const (
	DuplicateTopicIdThresholdKey           = "duplicate-topic-id-threshold"
	MaxTotalDuplicateTopicIdThresholdKey   = "max-total-duplicate-topic-id-threshold"
	MaxTotalDuplicateMessageIdThresholdKey = "max-total-duplicate-message-id-threshold"
)

// IHaveRpcInspectionParameters contains the "numerical values" for ihave rpc control inspection.
type IHaveRpcInspectionParameters struct {
	// MaxSampleSize max inspection sample size to use. If the number of ihave messages exceeds this configured value
	// the control message ihaves will be truncated to the max sample size. This sample is randomly selected.
	MaxSampleSize int `validate:"gte=1000" mapstructure:"max-sample-size"`
	// MaxMessageIDSampleSize max inspection sample size to use for iHave message ids. Each ihave message includes a list of message ids
	// each, if the size of this list exceeds the configured max message id sample size the list of message ids will be truncated.
	MaxMessageIDSampleSize int `validate:"gte=1000" mapstructure:"max-message-id-sample-size"`

	// DuplicateTopicIdThreshold is the threshold for considering the repeated topic IDs in a single iHave message as a duplicate.
	// For example, if the threshold is 2, a maximum of two duplicate topic ids will be allowed in a single iHave message.
	// This is to allow GossipSub protocol send iHave messages in batches without consolidating the topic IDs.
	DuplicateTopicIdThreshold int `validate:"gte=0" mapstructure:"duplicate-topic-id-threshold"`

	// MaxTotalDuplicateTopicIdThreshold is the tolerance threshold for having duplicate topics in an iHave message under inspection.
	// When the total number of duplicate topic ids in a single iHave message exceeds this threshold, the inspection of message will fail.
	// Note that a topic ID is counted as a duplicate only if it is repeated more than DuplicateTopicIdThreshold times.
	MaxTotalDuplicateTopicIdThreshold int `validate:"gte=0" mapstructure:"max-total-duplicate-topic-id-threshold"`

	// MaxTotalDuplicateMessageIdThreshold is the threshold of tolerance for having duplicate message IDs in a single iHave message under inspection.
	// When the total number of duplicate message ids in a single iHave message exceeds this threshold, the inspection of message will fail.
	// Ideally, an iHave message should not have any duplicate message IDs, hence a message id is considered duplicate when it is repeated more than once
	// within the same iHave message. When the total number of duplicate message ids in a single iHave message exceeds this threshold, the inspection of message will fail.
	MaxTotalDuplicateMessageIdThreshold int `validate:"gte=0" mapstructure:"max-total-duplicate-message-id-threshold"`
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
