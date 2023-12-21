package netconf

import (
	"fmt"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	// All constant strings are used for CLI flag names and corresponding keys for config values.
	// network configuration
	networkingConnectionPruning               = "networking-connection-pruning"
	preferredUnicastsProtocols                = "preferred-unicast-protocols"
	receivedMessageCacheSize                  = "received-message-cache-size"
	peerUpdateInterval                        = "peerupdate-interval"
	unicastMessageTimeout                     = "unicast-message-timeout"
	unicastCreateStreamRetryDelay             = "unicast-create-stream-retry-delay"
	unicastStreamZeroRetryResetThreshold      = "unicast-stream-zero-retry-reset-threshold"
	unicastDialZeroRetryResetThreshold        = "unicast-dial-zero-retry-reset-threshold"
	unicastMaxDialRetryAttemptTimes           = "unicast-max-dial-retry-attempt-times"
	unicastMaxStreamCreationRetryAttemptTimes = "unicast-max-stream-creation-retry-attempt-times"
	unicastDialInProgressBackoffDelay         = "unicast-dial-in-progress-backoff-delay"
	unicastDialBackoffDelay                   = "unicast-dial-backoff-delay"
	unicastDialConfigCacheSize                = "unicast-dial-config-cache-size"
	dnsCacheTTL                               = "dns-cache-ttl"
	disallowListNotificationCacheSize         = "disallow-list-notification-cache-size"
	// unicast rate limiters config
	dryRun              = "unicast-dry-run"
	lockoutDuration     = "unicast-lockout-duration"
	messageRateLimit    = "unicast-message-rate-limit"
	bandwidthRateLimit  = "unicast-bandwidth-rate-limit"
	bandwidthBurstLimit = "unicast-bandwidth-burst-limit"
	// resource manager config
	memoryLimitRatio               = "libp2p-memory-limit-ratio"
	fileDescriptorsRatio           = "libp2p-file-descriptors-ratio"
	peerBaseLimitConnsInbound      = "libp2p-peer-base-limits-conns-inbound"
	inboundStreamLimitSystem       = "libp2p-inbound-stream-limit-system"
	inboundStreamLimitPeer         = "libp2p-inbound-stream-limit-peer"
	inboundStreamLimitProtocol     = "libp2p-inbound-stream-limit-protocol"
	inboundStreamLimitProtocolPeer = "libp2p-inbound-stream-limit-protocol-peer"
	inboundStreamLimitTransient    = "libp2p-inbound-stream-limit-transient"

	// connection manager
	highWatermark = "libp2p-high-watermark"
	lowWatermark  = "libp2p-low-watermark"
	gracePeriod   = "libp2p-grace-period"
	silencePeriod = "libp2p-silence-period"
	// gossipsub
	peerScoring                  = "gossipsub-peer-scoring-enabled"
	localMeshLogInterval         = "gossipsub-local-mesh-logging-interval"
	rpcSentTrackerCacheSize      = "gossipsub-rpc-sent-tracker-cache-size"
	rpcSentTrackerQueueCacheSize = "gossipsub-rpc-sent-tracker-queue-cache-size"
	rpcSentTrackerNumOfWorkers   = "gossipsub-rpc-sent-tracker-workers"
	scoreTracerInterval          = "gossipsub-score-tracer-interval"

	// gossipsub validation inspector
	gossipSubRPCInspectorNotificationCacheSize                 = "gossipsub-rpc-inspector-notification-cache-size"
	validationInspectorNumberOfWorkers                         = "gossipsub-rpc-validation-inspector-workers"
	validationInspectorInspectMessageQueueCacheSize            = "gossipsub-rpc-validation-inspector-queue-cache-size"
	validationInspectorClusterPrefixedTopicsReceivedCacheSize  = "gossipsub-cluster-prefix-tracker-cache-size"
	validationInspectorClusterPrefixedTopicsReceivedCacheDecay = "gossipsub-cluster-prefix-tracker-cache-decay"
	validationInspectorClusterPrefixHardThreshold              = "gossipsub-rpc-cluster-prefixed-hard-threshold"

	ihaveMaxSampleSize           = "gossipsub-rpc-ihave-max-sample-size"
	ihaveMaxMessageIDSampleSize  = "gossipsub-rpc-ihave-max-message-id-sample-size"
	controlMessageMaxSampleSize  = "gossipsub-rpc-graft-and-prune-message-max-sample-size"
	iwantMaxSampleSize           = "gossipsub-rpc-iwant-max-sample-size"
	iwantMaxMessageIDSampleSize  = "gossipsub-rpc-iwant-max-message-id-sample-size"
	iwantCacheMissThreshold      = "gossipsub-rpc-iwant-cache-miss-threshold"
	iwantCacheMissCheckSize      = "gossipsub-rpc-iwant-cache-miss-check-size"
	iwantDuplicateMsgIDThreshold = "gossipsub-rpc-iwant-duplicate-message-id-threshold"

	// gossipsub metrics inspector
	metricsInspectorNumberOfWorkers = "gossipsub-rpc-metrics-inspector-workers"
	metricsInspectorCacheSize       = "gossipsub-rpc-metrics-inspector-cache-size"

	alspDisabled            = "alsp-disable-penalty"
	alspSpamRecordCacheSize = "alsp-spam-record-cache-size"
	alspSpamRecordQueueSize = "alsp-spam-report-queue-size"
	alspHearBeatInterval    = "alsp-heart-beat-interval"

	alspSyncEngineBatchRequestBaseProb = "alsp-sync-engine-batch-request-base-prob"
	alspSyncEngineRangeRequestBaseProb = "alsp-sync-engine-range-request-base-prob"
	alspSyncEngineSyncRequestProb      = "alsp-sync-engine-sync-request-prob"
)

func AllFlagNames() []string {
	return []string{
		networkingConnectionPruning,
		preferredUnicastsProtocols,
		receivedMessageCacheSize,
		peerUpdateInterval,
		unicastMessageTimeout,
		unicastCreateStreamRetryDelay,
		unicastDialInProgressBackoffDelay,
		unicastDialBackoffDelay,
		unicastStreamZeroRetryResetThreshold,
		unicastDialZeroRetryResetThreshold,
		unicastMaxDialRetryAttemptTimes,
		unicastMaxStreamCreationRetryAttemptTimes,
		unicastDialConfigCacheSize,
		dnsCacheTTL,
		disallowListNotificationCacheSize,
		dryRun,
		lockoutDuration,
		messageRateLimit,
		bandwidthRateLimit,
		bandwidthBurstLimit,
		memoryLimitRatio,
		fileDescriptorsRatio,
		peerBaseLimitConnsInbound,
		inboundStreamLimitSystem,
		inboundStreamLimitPeer,
		inboundStreamLimitProtocol,
		inboundStreamLimitProtocolPeer,
		inboundStreamLimitTransient,
		highWatermark,
		lowWatermark,
		gracePeriod,
		silencePeriod,
		peerScoring,
		localMeshLogInterval,
		rpcSentTrackerCacheSize,
		rpcSentTrackerQueueCacheSize,
		rpcSentTrackerNumOfWorkers,
		scoreTracerInterval,
		gossipSubRPCInspectorNotificationCacheSize,
		validationInspectorNumberOfWorkers,
		validationInspectorInspectMessageQueueCacheSize,
		validationInspectorClusterPrefixedTopicsReceivedCacheSize,
		validationInspectorClusterPrefixedTopicsReceivedCacheDecay,
		validationInspectorClusterPrefixHardThreshold,
		ihaveMaxSampleSize,
		metricsInspectorNumberOfWorkers,
		metricsInspectorCacheSize,
		alspDisabled,
		alspSpamRecordCacheSize,
		alspSpamRecordQueueSize,
		alspHearBeatInterval,
		alspSyncEngineBatchRequestBaseProb,
		alspSyncEngineRangeRequestBaseProb,
		alspSyncEngineSyncRequestProb,
		iwantMaxSampleSize,
		iwantMaxMessageIDSampleSize,
		ihaveMaxMessageIDSampleSize,
		iwantCacheMissThreshold,
		controlMessageMaxSampleSize,
		iwantDuplicateMsgIDThreshold,
		iwantCacheMissCheckSize,
	}
}

// InitializeNetworkFlags initializes all CLI flags for the Flow network configuration on the provided pflag set.
// Args:
//
//	*pflag.FlagSet: the pflag set of the Flow node.
//	*Config: the default network config used to set default values on the flags
func InitializeNetworkFlags(flags *pflag.FlagSet, config *Config) {
	flags.Bool(networkingConnectionPruning, config.NetworkConnectionPruning, "enabling connection trimming")
	flags.Duration(dnsCacheTTL, config.DNSCacheTTL, "time-to-live for dns cache")
	flags.StringSlice(preferredUnicastsProtocols, config.PreferredUnicastProtocols, "preferred unicast protocols in ascending order of preference")
	flags.Uint32(receivedMessageCacheSize, config.NetworkReceivedMessageCacheSize, "incoming message cache size at networking layer")
	flags.Uint32(disallowListNotificationCacheSize, config.DisallowListNotificationCacheSize, "cache size for notification events from disallow list")
	flags.Duration(peerUpdateInterval, config.PeerUpdateInterval, "how often to refresh the peer connections for the node")
	flags.Duration(unicastMessageTimeout, config.UnicastMessageTimeout, "how long a unicast transmission can take to complete")
	// unicast manager options
	flags.Duration(unicastCreateStreamRetryDelay,
		config.UnicastConfig.CreateStreamBackoffDelay,
		"initial backoff delay between failing to establish a connection with another node and retrying, "+
			"this delay increases exponentially with the number of subsequent failures to establish a connection.")
	flags.Duration(unicastDialBackoffDelay,
		config.UnicastConfig.DialInProgressBackoffDelay,
		"initial backoff delay between failing to establish a connection with another node and retrying, "+
			"this delay increases exponentially with the number of subsequent failures to establish a connection.")
	flags.Duration(unicastDialInProgressBackoffDelay,
		config.UnicastConfig.DialInProgressBackoffDelay,
		"initial backoff delay for concurrent stream creations to a remote peer when there is no exising connection and a dial is in progress. "+
			"this delay increases exponentially with the number of subsequent failure attempts")
	flags.Uint64(unicastStreamZeroRetryResetThreshold,
		config.UnicastConfig.StreamZeroRetryResetThreshold,
		"reset stream creation retry budget from zero to the maximum after consecutive successful streams reach this threshold.")
	flags.Duration(unicastDialZeroRetryResetThreshold,
		config.UnicastConfig.DialZeroRetryResetThreshold,
		"reset dial retry budget if the last successful dial is longer than this threshold.")
	flags.Uint64(unicastMaxDialRetryAttemptTimes, config.UnicastConfig.MaxDialRetryAttemptTimes, "maximum attempts to establish a unicast connection.")
	flags.Uint64(unicastMaxStreamCreationRetryAttemptTimes, config.UnicastConfig.MaxStreamCreationRetryAttemptTimes, "max attempts to create a unicast stream.")
	flags.Uint32(unicastDialConfigCacheSize,
		config.UnicastConfig.DialConfigCacheSize,
		"cache size of the dial config cache, recommended to be big enough to accommodate the entire nodes in the network.")

	// unicast stream handler rate limits
	flags.Int(messageRateLimit, config.UnicastConfig.UnicastRateLimitersConfig.MessageRateLimit, "maximum number of unicast messages that a peer can send per second")
	flags.Int(bandwidthRateLimit,
		config.UnicastConfig.UnicastRateLimitersConfig.BandwidthRateLimit,
		"bandwidth size in bytes a peer is allowed to send via unicast streams per second")
	flags.Int(bandwidthBurstLimit, config.UnicastConfig.UnicastRateLimitersConfig.BandwidthBurstLimit, "bandwidth size in bytes a peer is allowed to send at one time")
	flags.Duration(lockoutDuration,
		config.UnicastConfig.UnicastRateLimitersConfig.LockoutDuration,
		"the number of seconds a peer will be forced to wait before being allowed to successful reconnect to the node after being rate limited")
	flags.Bool(dryRun, config.UnicastConfig.UnicastRateLimitersConfig.DryRun, "disable peer disconnects and connections gating when rate limiting peers")
	// resource manager cli flags
	flags.Float64(fileDescriptorsRatio, config.ResourceManagerConfig.FileDescriptorsRatio, "ratio of available file descriptors to be used by libp2p (in (0,1])")
	flags.Float64(memoryLimitRatio, config.ResourceManagerConfig.MemoryLimitRatio, "ratio of available memory to be used by libp2p (in (0,1])")
	flags.Int(peerBaseLimitConnsInbound, config.ResourceManagerConfig.PeerBaseLimitConnsInbound, "the maximum amount of allowed inbound connections per peer")
	flags.Int(inboundStreamLimitSystem, config.ResourceManagerConfig.InboundStream.System, "the system-wide limit on the number of inbound streams")
	flags.Int(inboundStreamLimitPeer, config.ResourceManagerConfig.InboundStream.Peer, "the limit on the number of inbound streams per peer (over all protocols)")
	flags.Int(inboundStreamLimitProtocol, config.ResourceManagerConfig.InboundStream.Protocol, "the limit on the number of inbound streams per protocol (over all peers)")
	flags.Int(inboundStreamLimitProtocolPeer, config.ResourceManagerConfig.InboundStream.ProtocolPeer, "the limit on the number of inbound streams per protocol per peer")
	flags.Int(inboundStreamLimitTransient,
		config.ResourceManagerConfig.InboundStream.Transient,
		"the transient limit on the number of inbound streams (applied to streams that are not associated with a peer or protocol yet)")

	// connection manager
	flags.Int(lowWatermark, config.ConnectionManagerConfig.LowWatermark, "low watermarking for libp2p connection manager")
	flags.Int(highWatermark, config.ConnectionManagerConfig.HighWatermark, "high watermarking for libp2p connection manager")
	flags.Duration(gracePeriod, config.ConnectionManagerConfig.GracePeriod, "grace period for libp2p connection manager")
	flags.Duration(silencePeriod, config.ConnectionManagerConfig.SilencePeriod, "silence period for libp2p connection manager")
	flags.Bool(peerScoring, config.GossipSubConfig.PeerScoring, "enabling peer scoring on pubsub network")
	flags.Duration(localMeshLogInterval, config.GossipSubConfig.LocalMeshLogInterval, "logging interval for local mesh in gossipsub")
	flags.Duration(scoreTracerInterval, config.GossipSubConfig.ScoreTracerInterval, "logging interval for peer score tracer in gossipsub, set to 0 to disable")
	flags.Uint32(rpcSentTrackerCacheSize, config.GossipSubConfig.RPCSentTrackerCacheSize, "cache size of the rpc sent tracker used by the gossipsub mesh tracer.")
	flags.Uint32(rpcSentTrackerQueueCacheSize, config.GossipSubConfig.RPCSentTrackerQueueCacheSize, "cache size of the rpc sent tracker worker queue.")
	flags.Int(rpcSentTrackerNumOfWorkers, config.GossipSubConfig.RpcSentTrackerNumOfWorkers, "number of workers for the rpc sent tracker worker pool.")
	// gossipsub RPC control message validation limits used for validation configuration and rate limiting
	flags.Int(validationInspectorNumberOfWorkers,
		config.GossipSubConfig.GossipSubRPCInspectorsConfig.GossipSubRPCValidationInspectorConfigs.NumberOfWorkers,
		"number of gossupsub RPC control message validation inspector component workers")
	flags.Uint32(validationInspectorInspectMessageQueueCacheSize,
		config.GossipSubConfig.GossipSubRPCInspectorsConfig.GossipSubRPCValidationInspectorConfigs.CacheSize,
		"cache size for gossipsub RPC validation inspector events worker pool queue.")
	flags.Uint32(validationInspectorClusterPrefixedTopicsReceivedCacheSize,
		config.GossipSubConfig.GossipSubRPCInspectorsConfig.GossipSubRPCValidationInspectorConfigs.ClusterPrefixedControlMsgsReceivedCacheSize,
		"cache size for gossipsub RPC validation inspector cluster prefix received tracker.")
	flags.Float64(validationInspectorClusterPrefixedTopicsReceivedCacheDecay,
		config.GossipSubConfig.GossipSubRPCInspectorsConfig.GossipSubRPCValidationInspectorConfigs.ClusterPrefixedControlMsgsReceivedCacheDecay,
		"the decay value used to decay cluster prefix received topics received cached counters.")
	flags.Float64(validationInspectorClusterPrefixHardThreshold,
		config.GossipSubConfig.GossipSubRPCInspectorsConfig.GossipSubRPCValidationInspectorConfigs.ClusterPrefixHardThreshold,
		"the maximum number of cluster-prefixed control messages allowed to be processed when the active cluster id is unset or a mismatch is detected, exceeding this threshold will result in node penalization by gossipsub.")
	// networking event notifications
	flags.Uint32(gossipSubRPCInspectorNotificationCacheSize,
		config.GossipSubConfig.GossipSubRPCInspectorsConfig.GossipSubRPCInspectorNotificationCacheSize,
		"cache size for notification events from gossipsub rpc inspector")
	// application layer spam prevention (alsp) protocol
	flags.Bool(alspDisabled, config.AlspConfig.DisablePenalty, "disable the penalty mechanism of the alsp protocol. default value (recommended) is false")
	flags.Uint32(alspSpamRecordCacheSize, config.AlspConfig.SpamRecordCacheSize, "size of spam record cache, recommended to be 10x the number of authorized nodes")
	flags.Uint32(alspSpamRecordQueueSize, config.AlspConfig.SpamReportQueueSize, "size of spam report queue, recommended to be 100x the number of authorized nodes")
	flags.Duration(alspHearBeatInterval, config.AlspConfig.HearBeatInterval, "interval between two consecutive heartbeat events at alsp, recommended to leave it as default unless you know what you are doing.")
	flags.Float32(alspSyncEngineBatchRequestBaseProb, config.AlspConfig.SyncEngine.BatchRequestBaseProb, "base probability of creating a misbehavior report for a batch request message")
	flags.Float32(alspSyncEngineRangeRequestBaseProb, config.AlspConfig.SyncEngine.RangeRequestBaseProb, "base probability of creating a misbehavior report for a range request message")
	flags.Float32(alspSyncEngineSyncRequestProb, config.AlspConfig.SyncEngine.SyncRequestProb, "probability of creating a misbehavior report for a sync request message")

	flags.Int(ihaveMaxSampleSize,
		config.GossipSubConfig.GossipSubRPCInspectorsConfig.GossipSubRPCValidationInspectorConfigs.IHaveRPCInspectionConfig.MaxSampleSize,
		"max number of ihaves to sample when performing validation")
	flags.Int(ihaveMaxMessageIDSampleSize,
		config.GossipSubConfig.GossipSubRPCInspectorsConfig.GossipSubRPCValidationInspectorConfigs.IHaveRPCInspectionConfig.MaxMessageIDSampleSize,
		"max number of message ids to sample when performing validation per ihave")
	flags.Int(controlMessageMaxSampleSize,
		config.GossipSubConfig.GossipSubRPCInspectorsConfig.GossipSubRPCValidationInspectorConfigs.GraftPruneMessageMaxSampleSize,
		"max number of control messages to sample when performing validation on GRAFT and PRUNE message types")
	flags.Uint(iwantMaxSampleSize,
		config.GossipSubConfig.GossipSubRPCInspectorsConfig.GossipSubRPCValidationInspectorConfigs.IWantRPCInspectionConfig.MaxSampleSize,
		"max number of iwants to sample when performing validation")
	flags.Int(iwantMaxMessageIDSampleSize,
		config.GossipSubConfig.GossipSubRPCInspectorsConfig.GossipSubRPCValidationInspectorConfigs.IWantRPCInspectionConfig.MaxMessageIDSampleSize,
		"max number of message ids to sample when performing validation per iwant")
	flags.Float64(iwantCacheMissThreshold,
		config.GossipSubConfig.GossipSubRPCInspectorsConfig.GossipSubRPCValidationInspectorConfigs.IWantRPCInspectionConfig.CacheMissThreshold,
		"max number of iwants to sample when performing validation")
	flags.Int(iwantCacheMissCheckSize,
		config.GossipSubConfig.GossipSubRPCInspectorsConfig.GossipSubRPCValidationInspectorConfigs.IWantRPCInspectionConfig.CacheMissCheckSize,
		"the iWants size at which message id cache misses will be checked")
	flags.Float64(iwantDuplicateMsgIDThreshold,
		config.GossipSubConfig.GossipSubRPCInspectorsConfig.GossipSubRPCValidationInspectorConfigs.IWantRPCInspectionConfig.DuplicateMsgIDThreshold,
		"max allowed duplicate message IDs in a single iWant control message")
}

// SetAliases this func sets an aliases for each CLI flag defined for network config overrides to it's corresponding
// full key in the viper config store. This is required because in our config.yml file all configuration values for the
// Flow network are stored one level down on the network-config property. When the default config is bootstrapped viper will
// store these values with the "network-config." prefix on the config key, because we do not want to use CLI flags like --network-config.networking-connection-pruning
// to override default values we instead use cleans flags like --networking-connection-pruning and create an alias from networking-connection-pruning -> network-config.networking-connection-pruning
// to ensure overrides happen as expected.
// Args:
// *viper.Viper: instance of the viper store to register network config aliases on.
// Returns:
// error: if a flag does not have a corresponding key in the viper store.
func SetAliases(conf *viper.Viper) error {
	m := make(map[string]string)
	// create map of key -> full pathkey
	// ie: "networking-connection-pruning" -> "network-config.networking-connection-pruning"
	for _, key := range conf.AllKeys() {
		s := strings.Split(key, ".")
		// check len of s, we expect all network keys to have a single prefix "network-config"
		// s should always contain only 2 elements
		if len(s) == 2 {
			m[s[1]] = key
		}
	}
	// each flag name should correspond to exactly one key in our config store after it is loaded with the default config
	for _, flagName := range AllFlagNames() {
		fullKey, ok := m[flagName]
		if !ok {
			return fmt.Errorf("invalid network configuration missing configuration key flag name %s check config file and cli flags", flagName)
		}
		conf.RegisterAlias(fullKey, flagName)
	}
	return nil
}
