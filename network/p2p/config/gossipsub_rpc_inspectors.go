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
	ProcessKey                      = "process"
	ClusterPrefixedMessageConfigKey = "cluster-prefixed-messages"
	IWantConfigKey                  = "iwant"
	IHaveConfigKey                  = "ihave"
	GraftPruneKey                   = "graft-and-prune"
	PublishMessagesConfigKey        = "publish-messages"
	InspectionQueueConfigKey        = "inspection-queue"
)

// RpcValidationInspector rpc control message validation inspector configuration.
type RpcValidationInspector struct {
	ClusterPrefixedMessage ClusterPrefixedMessageInspectionParameters `mapstructure:"cluster-prefixed-messages"`
	IWant                  IWantRpcInspectionParameters               `mapstructure:"iwant"`
	IHave                  IHaveRpcInspectionParameters               `mapstructure:"ihave"`
	GraftPrune             GraftPruneRpcInspectionParameters          `mapstructure:"graft-and-prune"`
	PublishMessages        PublishMessageInspectionParameters         `mapstructure:"publish-messages"`
	InspectionQueue        InspectionQueueParameters                  `mapstructure:"inspection-queue"`
	// InspectionProcess configuration that controls which aspects of rpc inspection are enabled and disabled during inspect message request processing.
	InspectionProcess InspectionProcess `mapstructure:"process"`
}

// InspectionProcess configuration that controls which aspects of rpc inspection are enabled and disabled during inspect message request processing.
type InspectionProcess struct {
	Inspect  Inspect  `validate:"required" mapstructure:"inspection"`
	Truncate Truncate `validate:"required" mapstructure:"truncation"`
}

const (
	InspectionKey = "inspection"
	TruncationKey = "truncation"
	EnableKey     = "enable"
	DisabledKey   = "disabled"
	MessageIDKey  = "message-id"
)

// Inspect configuration to enable/disable RPC inspection for a particular control message type.
type Inspect struct {
	// Disabled serves as a fail-safe mechanism to globally deactivate inspection logic. When this fail-safe is activated it disables all
	// aspects of the inspection logic, irrespective of individual configurations like inspection.enable-graft, inspection.enable-prune, etc.
	// Consequently, all metrics collection and logging related to the rpc and inspection will also be disabled.
	// It is important to note that activating this fail-safe results in a comprehensive deactivation inspection features.
	// Please use this setting judiciously, considering its broad impact on the behavior of control message handling.
	Disabled bool `mapstructure:"disabled"`
	// EnableGraft enable graft control message inspection.
	EnableGraft bool `mapstructure:"enable-graft"`
	// EnablePrune enable prune control message inspection.
	EnablePrune bool `mapstructure:"enable-prune"`
	// EnableIHave enable iHave control message inspection.
	EnableIHave bool `mapstructure:"enable-ihave"`
	// EnableIWant enable iWant control message inspection.
	EnableIWant bool `mapstructure:"enable-iwant"`
	// EnablePublish enable publish message inspection.
	EnablePublish bool `mapstructure:"enable-publish"`
}

// Truncate configuration to enable/disable RPC truncation for a particular control message type.
type Truncate struct {
	// Disabled serves as a fail-safe mechanism to globally deactivate truncation logic. When this fail-safe is activated it disables all
	// aspects of the truncation logic, irrespective of individual configurations like truncation.enable-graft, truncation.enable-prune, etc.
	// Consequently, all metrics collection and logging related to the rpc and inspection will also be disabled.
	// It is important to note that activating this fail-safe results in a comprehensive deactivation truncation features.
	// Please use this setting judiciously, considering its broad impact on the behavior of control message handling.
	Disabled bool `mapstructure:"disabled"`
	// EnableGraft enable graft control message truncation.
	EnableGraft bool `mapstructure:"enable-graft"`
	// EnablePrune enable prune control message truncation.
	EnablePrune bool `mapstructure:"enable-prune"`
	// EnableIHave enable iHave control message truncation.
	EnableIHave bool `mapstructure:"enable-ihave"`
	// EnableIHaveMessageIds enable iHave message id truncation.
	EnableIHaveMessageIds bool `mapstructure:"enable-ihave-message-id"`
	// EnableIWant enable iWant control message truncation.
	EnableIWant bool `mapstructure:"enable-iwant"`
	// EnableIWantMessageIds enable iWant message id truncation.
	EnableIWantMessageIds bool `mapstructure:"enable-iwant-message-id"`
}

const (
	QueueSizeKey = "queue-size"
)

// InspectionQueueParameters contains the "numerical values" for the control message validation inspector.
// Incoming GossipSub RPCs are queued for async inspection by a worker pool. This worker pool is configured
// by the parameters in this struct.
// Each RPC has a number of "publish messages" accompanied by control messages.
type InspectionQueueParameters struct {
	// NumberOfWorkers number of worker pool workers.
	NumberOfWorkers int `validate:"gte=1" mapstructure:"workers"`
	// Size size of the queue used by worker pool for the control message validation inspector.
	Size uint32 `validate:"gte=100" mapstructure:"queue-size"`
}

const (
	MaxSampleSizeKey         = "max-sample-size"
	MessageErrorThresholdKey = "error-threshold"
)

// PublishMessageInspectionParameters contains the "numerical values" for the publish control message inspection.
// Each RPC has a number of "publish messages" accompanied by control messages. This struct contains the limits
// for the inspection of these publish messages.
type PublishMessageInspectionParameters struct {
	// MaxSampleSize is the maximum number of messages in a single RPC message that are randomly sampled for async inspection.
	// When the size of a single RPC message exceeds this threshold, a random sample is taken for inspection, but the RPC message is not truncated.
	MaxSampleSize int `validate:"gte=0" mapstructure:"max-sample-size"`
	// ErrorThreshold the threshold at which an error will be returned if the number of invalid RPC messages exceeds this value.
	ErrorThreshold int `validate:"gte=0" mapstructure:"error-threshold"`
}

// GraftPruneRpcInspectionParameters contains the "numerical values" for the graft and prune control message inspection.
// Each RPC has a number of "publish messages" accompanied by control messages. This struct contains the limits
// for the inspection of these graft and prune control messages.
type GraftPruneRpcInspectionParameters struct {
	// MessageCountThreshold is the maximum number of GRAFT or PRUNE messages in a single RPC message.
	// When the total number of GRAFT or PRUNE messages in a single RPC message exceeds this threshold,
	// a random sample of GRAFT or PRUNE messages will be taken and the RPC message will be truncated to this sample size.
	MessageCountThreshold int `validate:"gte=0" mapstructure:"message-count-threshold"`

	// DuplicateTopicIdThreshold is the tolerance threshold for having duplicate topics in a single GRAFT or PRUNE message under inspection.
	// Ideally, a GRAFT or PRUNE message should not have any duplicate topics, hence a topic ID is counted as a duplicate only if it is repeated more than once.
	// When the total number of duplicate topic ids in a single GRAFT or PRUNE message exceeds this threshold, the inspection of message will fail.
	DuplicateTopicIdThreshold int `validate:"gte=0" mapstructure:"duplicate-topic-id-threshold"`

	// InvalidTopicIdThreshold Maximum number of total invalid topic ids in a single GRAFT or PRUNE message, ideally this should be 0 but we allow for some tolerance
	// to avoid penalizing peers that are not malicious but are misbehaving due to bugs or other issues.
	InvalidTopicIdThreshold int `validate:"gte=0" mapstructure:"invalid-topic-id-threshold"`
}

const (
	MessageCountThreshold      = "message-count-threshold"
	MessageIdCountThreshold    = "message-id-count-threshold"
	CacheMissThresholdKey      = "cache-miss-threshold"
	DuplicateMsgIDThresholdKey = "duplicate-message-id-threshold"
	InvalidTopicIdThresholdKey = "invalid-topic-id-threshold"
)

// IWantRpcInspectionParameters contains the "numerical values" for iwant rpc control inspection.
// Each RPC has a number of "publish messages" accompanied by control messages. This struct contains the limits
// for the inspection of the iwant control messages.
type IWantRpcInspectionParameters struct {
	// MessageCountThreshold is the maximum allowed number of iWant messages in a single RPC message.
	// Each iWant message represents the list of message ids. When the total number of iWant messages
	// in a single RPC message exceeds this threshold, a random sample of iWant messages will be taken and the RPC message will be truncated to this sample size.
	// The sample size is equal to the configured MessageCountThreshold.
	MessageCountThreshold uint `validate:"gt=0" mapstructure:"message-count-threshold"`
	// MessageIdCountThreshold is the maximum allowed number of message ids in a single iWant message.
	// Each iWant message represents the list of message ids for a specific topic, and this parameter controls the maximum number of message ids
	// that can be included in a single iWant message. When the total number of message ids in a single iWant message exceeds this threshold,
	// a random sample of message ids will be taken and the iWant message will be truncated to this sample size.
	// The sample size is equal to the configured MessageIdCountThreshold.
	MessageIdCountThreshold int `validate:"gte=0" mapstructure:"message-id-count-threshold"`
	// CacheMissThreshold is the threshold of tolerance for the total cache misses in all iWant messages in a single RPC message.
	// When the total number of cache misses in all iWant messages in a single RPC message exceeds this threshold, the inspection of message will fail.
	// An iWant message is considered a cache miss if it contains a message id that is not present in the local cache for iHave messages, i.e., the node
	// does not have a record of an iHave message for this message id.
	// When the total number of cache misses in all iWant messages in a single RPC message exceeds this threshold, the inspection of message will fail, and
	// a single misbehavior notification will be reported.
	CacheMissThreshold int `validate:"gt=0" mapstructure:"cache-miss-threshold"`
	// DuplicateMsgIdThreshold is the maximum allowed number of duplicate message ids in a all iWant messages in a single RPC message.
	// Each iWant message represents the list of message ids, and this parameter controls the maximum number of duplicate message ids
	// that can be included in all iWant messages in a single RPC message. When the total number of duplicate message ids in a single iWant message exceeds this threshold,
	// a single misbehavior notification will be reported, and the inspection of message will fail.
	DuplicateMsgIdThreshold int `validate:"gt=0" mapstructure:"duplicate-message-id-threshold"`
}

const (
	DuplicateTopicIdThresholdKey   = "duplicate-topic-id-threshold"
	DuplicateMessageIdThresholdKey = "duplicate-message-id-threshold"
)

// IHaveRpcInspectionParameters contains the "numerical values" for ihave rpc control inspection.
// Each RPC has a number of "publish messages" accompanied by control messages. This struct contains the limits
// for the inspection of the ihave control messages.
type IHaveRpcInspectionParameters struct {
	// MessageCountThreshold is the maximum allowed number of iHave messages in a single RPC message.
	// Each iHave message represents the list of message ids for a specific topic. When the total number of iHave messages
	// in a single RPC message exceeds this threshold, a random sample of iHave messages will be taken and the RPC message will be truncated to this sample size.
	// The sample size is equal to the configured MessageCountThreshold.
	MessageCountThreshold int `validate:"gte=0" mapstructure:"message-count-threshold"`
	// MessageIdCountThreshold is the maximum allowed number of message ids in a single iHave message.
	// Each iHave message represents the list of message ids for a specific topic, and this parameter controls the maximum number of message ids
	// that can be included in a single iHave message. When the total number of message ids in a single iHave message exceeds this threshold,
	// a random sample of message ids will be taken and the iHave message will be truncated to this sample size.
	// The sample size is equal to the configured MessageIdCountThreshold.
	MessageIdCountThreshold int `validate:"gte=0" mapstructure:"message-id-count-threshold"`

	// DuplicateTopicIdThreshold is the tolerance threshold for having duplicate topics in an iHave message under inspection.
	// When the total number of duplicate topic ids in a single iHave message exceeds this threshold, the inspection of message will fail.
	// Note that a topic ID is counted as a duplicate only if it is repeated more than DuplicateTopicIdThreshold times.
	DuplicateTopicIdThreshold int `validate:"gte=0" mapstructure:"duplicate-topic-id-threshold"`

	// DuplicateMessageIdThreshold is the threshold of tolerance for having duplicate message IDs in a single iHave message under inspection.
	// When the total number of duplicate message ids in a single iHave message exceeds this threshold, the inspection of message will fail.
	// Ideally, an iHave message should not have any duplicate message IDs, hence a message id is considered duplicate when it is repeated more than once
	// within the same iHave message. When the total number of duplicate message ids in a single iHave message exceeds this threshold, the inspection of message will fail.
	DuplicateMessageIdThreshold int `validate:"gte=0" mapstructure:"duplicate-message-id-threshold"`

	// InvalidTopicIdThreshold Maximum number of total invalid topic ids in a single IHAVE message, ideally this should be 0 but we allow for some tolerance
	// to avoid penalizing peers that are not malicious but are misbehaving due to bugs or other issues.
	InvalidTopicIdThreshold int `validate:"gte=0" mapstructure:"invalid-topic-id-threshold"`
}

const (
	HardThresholdKey     = "hard-threshold"
	TrackerCacheSizeKey  = "tracker-cache-size"
	TrackerCacheDecayKey = "tracker-cache-decay"
)

// ClusterPrefixedMessageInspectionParameters contains the "numerical values" for cluster prefixed control message inspection.
// Each RPC has a number of "publish messages" accompanied by control messages. This struct contains the limits for the inspection
// of messages (publish messages and control messages) that belongs to cluster prefixed topics.
// Cluster-prefixed topics are topics that are prefixed with the cluster ID of the node that published the message.
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
