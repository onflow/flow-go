package network

import (
	"fmt"
	"strings"

	"github.com/spf13/pflag"

	"github.com/onflow/flow-go/network/p2p"
)

const (
	// network configuration
	NetworkingConnectionPruning       = "networking-connection-pruning"
	PreferredUnicastsProtocols        = "preferred-unicast-protocols"
	ReceivedMessageCacheSize          = "received-message-cache-size"
	PeerUpdateInterval                = "peerupdate-interval"
	UnicastMessageTimeout             = "unicast-message-timeout"
	UnicastCreateStreamRetryDelay     = "unicast-create-stream-retry-delay"
	DnsCacheTTL                       = "dns-cache-ttl"
	DisallowListNotificationCacheSize = "disallow-list-notification-cache-size"
	// unicast rate limiters config
	DryRun              = "unicast-dry-run"
	LockoutDuration     = "unicast-lockout-duration"
	MessageRateLimit    = "unicast-message-rate-limit"
	BandwidthRateLimit  = "unicast-bandwidth-rate-limit"
	BandwidthBurstLimit = "unicast-bandwidth-burst-limit"
	// resource manager config
	MemoryLimitRatio          = "libp2p-memory-limit"
	FileDescriptorsRatio      = "libp2p-fd-ratio"
	PeerBaseLimitConnsInbound = "libp2p-inbound-conns-limit"
	// connection manager
	HighWatermark = "libp2p-connmgr-high"
	LowWatermark  = "libp2p-connmgr-low"
	GracePeriod   = "libp2p-connmgr-grace"
	SilencePeriod = "libp2p-connmgr-silence"
	// gossipsub
	PeerScoring          = "peer-scoring-enabled"
	LocalMeshLogInterval = "gossipsub-local-mesh-logging-interval"
	ScoreTracerInterval  = "gossipsub-score-tracer-interval"
	// gossipsub validation inspector
	GossipSubRPCInspectorNotificationCacheSize                 = "gossipsub-rpc-inspector-notification-cache-size"
	ValidationInspectorNumberOfWorkers                         = "gossipsub-rpc-validation-inspector-workers"
	ValidationInspectorInspectMessageQueueCacheSize            = "gossipsub-rpc-validation-inspector-queue-cache-size"
	ValidationInspectorClusterPrefixedTopicsReceivedCacheSize  = "gossipsub-cluster-prefix-tracker-cache-size"
	ValidationInspectorClusterPrefixedTopicsReceivedCacheDecay = "gossipsub-cluster-prefix-tracker-cache-decay"
	ValidationInspectorClusterPrefixHardThreshold              = "gossipsub-rpc-cluster-prefixed-hard-threshold"

	// gossipsub metrics inspector
	MetricsInspectorNumberOfWorkers = "gossipsub-rpc-metrics-inspector-workers"
	MetricsInspectorCacheSize       = "gossipsub-rpc-metrics-inspector-cache-size"

	ALSPDisabled            = "alsp-disable"
	ALSPSpamRecordCacheSize = "alsp-spam-record-cache-size"
	ALSPSpamRecordQueueSize = "alsp-spam-report-queue-size"
)

// InitializeNetworkFlags initializes all CLI flags for the Flow network configuration on the provided pflag set.
// Args:
//
//	*pflag.FlagSet: the pflag set of the Flow node.
//	*Config: the default network config used to set default values on the flags
func InitializeNetworkFlags(flags *pflag.FlagSet, defaultNetConfig *Config) {
	initRpcInspectorValidationLimitsFlags(flags, defaultNetConfig)
	flags.Bool(NetworkingConnectionPruning, defaultNetConfig.NetworkConnectionPruning, "enabling connection trimming")
	flags.Duration(DnsCacheTTL, defaultNetConfig.DNSCacheTTL, "time-to-live for dns cache")
	flags.StringSlice(PreferredUnicastsProtocols, defaultNetConfig.PreferredUnicastProtocols, "preferred unicast protocols in ascending order of preference")
	flags.Uint32(ReceivedMessageCacheSize, defaultNetConfig.NetworkReceivedMessageCacheSize, "incoming message cache size at networking layer")
	flags.Uint32(DisallowListNotificationCacheSize, defaultNetConfig.DisallowListNotificationCacheSize, "cache size for notification events from disallow list")
	flags.Duration(PeerUpdateInterval, defaultNetConfig.PeerUpdateInterval, "how often to refresh the peer connections for the node")
	flags.Duration(UnicastMessageTimeout, defaultNetConfig.UnicastMessageTimeout, "how long a unicast transmission can take to complete")
	// unicast manager options
	flags.Duration(UnicastCreateStreamRetryDelay, defaultNetConfig.UnicastCreateStreamRetryDelay, "Initial delay between failing to establish a connection with another node and retrying. This delay increases exponentially (exponential backoff) with the number of subsequent failures to establish a connection.")
	// unicast stream handler rate limits
	flags.Int(MessageRateLimit, defaultNetConfig.UnicastRateLimitersConfig.MessageRateLimit, "maximum number of unicast messages that a peer can send per second")
	flags.Int(BandwidthRateLimit, defaultNetConfig.UnicastRateLimitersConfig.BandwidthRateLimit, "bandwidth size in bytes a peer is allowed to send via unicast streams per second")
	flags.Int(BandwidthBurstLimit, defaultNetConfig.UnicastRateLimitersConfig.BandwidthBurstLimit, "bandwidth size in bytes a peer is allowed to send at one time")
	flags.Duration(LockoutDuration, defaultNetConfig.UnicastRateLimitersConfig.LockoutDuration, "the number of seconds a peer will be forced to wait before being allowed to successful reconnect to the node after being rate limited")
	flags.Bool(DryRun, defaultNetConfig.UnicastRateLimitersConfig.DryRun, "disable peer disconnects and connections gating when rate limiting peers")
	// resource manager cli flags
	flags.Float64(FileDescriptorsRatio, defaultNetConfig.LibP2PResourceManagerConfig.FileDescriptorsRatio, "ratio of available file descriptors to be used by libp2p (in (0,1])")
	flags.Float64(MemoryLimitRatio, defaultNetConfig.LibP2PResourceManagerConfig.MemoryLimitRatio, "ratio of available memory to be used by libp2p (in (0,1])")
	flags.Int(PeerBaseLimitConnsInbound, defaultNetConfig.LibP2PResourceManagerConfig.PeerBaseLimitConnsInbound, "the maximum amount of allowed inbound connections per peer")
	// connection manager
	flags.Int(LowWatermark, defaultNetConfig.ConnectionManagerConfig.LowWatermark, "low watermarking for libp2p connection manager")
	flags.Int(HighWatermark, defaultNetConfig.ConnectionManagerConfig.HighWatermark, "high watermarking for libp2p connection manager")
	flags.Duration(GracePeriod, defaultNetConfig.ConnectionManagerConfig.GracePeriod, "grace period for libp2p connection manager")
	flags.Duration(SilencePeriod, defaultNetConfig.ConnectionManagerConfig.SilencePeriod, "silence period for libp2p connection manager")
	flags.Bool(PeerScoring, defaultNetConfig.GossipSubConfig.PeerScoring, "enabling peer scoring on pubsub network")
	flags.Duration(LocalMeshLogInterval, defaultNetConfig.GossipSubConfig.LocalMeshLogInterval, "logging interval for local mesh in gossipsub")
	flags.Duration(ScoreTracerInterval, defaultNetConfig.GossipSubConfig.ScoreTracerInterval, "logging interval for peer score tracer in gossipsub, set to 0 to disable")
	// gossipsub RPC control message validation limits used for validation configuration and rate limiting
	flags.Int(ValidationInspectorNumberOfWorkers, defaultNetConfig.GossipSubConfig.RpcInspector.ValidationInspectorConfigs.NumberOfWorkers, "number of gossupsub RPC control message validation inspector component workers")
	flags.Uint32(ValidationInspectorInspectMessageQueueCacheSize, defaultNetConfig.GossipSubConfig.RpcInspector.ValidationInspectorConfigs.CacheSize, "cache size for gossipsub RPC validation inspector events worker pool queue.")
	flags.Uint32(ValidationInspectorClusterPrefixedTopicsReceivedCacheSize, defaultNetConfig.GossipSubConfig.RpcInspector.ValidationInspectorConfigs.ClusterPrefixedControlMsgsReceivedCacheSize, "cache size for gossipsub RPC validation inspector cluster prefix received tracker.")
	flags.Float64(ValidationInspectorClusterPrefixedTopicsReceivedCacheDecay, defaultNetConfig.GossipSubConfig.RpcInspector.ValidationInspectorConfigs.ClusterPrefixedControlMsgsReceivedCacheDecay, "the decay value used to decay cluster prefix received topics received cached counters.")
	flags.Float64(ValidationInspectorClusterPrefixHardThreshold, defaultNetConfig.GossipSubConfig.RpcInspector.ValidationInspectorConfigs.ClusterPrefixHardThreshold, "the maximum number of cluster-prefixed control messages allowed to be processed when the active cluster id is unset or a mismatch is detected, exceeding this threshold will result in node penalization by gossipsub.")
	// gossipsub RPC control message metrics observer inspector configuration
	flags.Int(MetricsInspectorNumberOfWorkers, defaultNetConfig.GossipSubConfig.RpcInspector.MetricsInspectorConfigs.NumberOfWorkers, "cache size for gossipsub RPC metrics inspector events worker pool queue.")
	flags.Uint32(MetricsInspectorCacheSize, defaultNetConfig.GossipSubConfig.RpcInspector.MetricsInspectorConfigs.CacheSize, "cache size for gossipsub RPC metrics inspector events worker pool.")
	// networking event notifications
	flags.Uint32(GossipSubRPCInspectorNotificationCacheSize, defaultNetConfig.GossipSubConfig.RpcInspector.GossipSubRPCInspectorNotificationCacheSize, "cache size for notification events from gossipsub rpc inspector")
	// application layer spam prevention (alsp) protocol
	flags.Bool(ALSPDisabled, defaultNetConfig.AlspConfig.DisablePenalty, "disable the penalty mechanism of the alsp protocol. default value (recommended) is false")
	flags.Uint32(ALSPSpamRecordCacheSize, defaultNetConfig.AlspConfig.SpamRecordCacheSize, "size of spam record cache, recommended to be 10x the number of authorized nodes")
	flags.Uint32(ALSPSpamRecordQueueSize, defaultNetConfig.AlspConfig.SpamReportQueueSize, "size of spam report queue, recommended to be 100x the number of authorized nodes")
}

// rpcInspectorValidationLimits utility func that adds flags for each of the validation limits for each control message type.
func initRpcInspectorValidationLimitsFlags(flags *pflag.FlagSet, defaultNetConfig *Config) {
	hardThresholdflagStrFmt := "gossipsub-rpc-%s-hard-threshold"
	safetyThresholdflagStrFmt := "gossipsub-rpc-%s-safety-threshold"
	rateLimitflagStrFmt := "gossipsub-rpc-%s-ratelimit"
	validationInspectorConfig := defaultNetConfig.GossipSubConfig.RpcInspector.ValidationInspectorConfigs

	for _, ctrlMsgType := range p2p.ControlMessageTypes() {
		if ctrlMsgType == p2p.CtrlMsgIWant {
			continue
		}

		var ctrlMsgValidationConfig *CtrlMsgValidationConfig
		switch ctrlMsgType {
		case p2p.CtrlMsgGraft:
			ctrlMsgValidationConfig = validationInspectorConfig.GraftLimits
		case p2p.CtrlMsgPrune:
			ctrlMsgValidationConfig = validationInspectorConfig.PruneLimits
		case p2p.CtrlMsgIHave:
			ctrlMsgValidationConfig = validationInspectorConfig.IHaveLimits
		}

		s := strings.ToLower(ctrlMsgType.String())
		flags.Uint64(fmt.Sprintf(hardThresholdflagStrFmt, s), ctrlMsgValidationConfig.HardThreshold, fmt.Sprintf("discard threshold limit for gossipsub RPC %s message validation", ctrlMsgType))
		flags.Uint64(fmt.Sprintf(safetyThresholdflagStrFmt, s), ctrlMsgValidationConfig.SafetyThreshold, fmt.Sprintf("safety threshold limit for gossipsub RPC %s message validation", ctrlMsgType))
		flags.Int(fmt.Sprintf(rateLimitflagStrFmt, s), ctrlMsgValidationConfig.RateLimit, fmt.Sprintf("rate limit for gossipsub RPC %s message validation", ctrlMsgType))
	}
}
