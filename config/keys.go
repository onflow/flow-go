package config

const (
	// network configuration keys
	NetworkingConnectionPruningKey       = "networking-connection-pruning"
	PreferredUnicastsProtocolsKey        = "preferred-unicasts-protocols"
	ReceivedMessageCacheSizeKey          = "received-message-cache-size"
	PeerUpdateIntervalKey                = "peer-update-interval"
	UnicastMessageTimeoutKey             = "unicast-message-timeout"
	UnicastCreateStreamRetryDelayKey     = "unicast-create-stream-retry-delay"
	DnsCacheTTLKey                       = "dns-cache-ttl"
	DisallowListNotificationCacheSizeKey = "disallow-list-notification-cache-size"
	// unicast rate limiters config keys
	DryRunKey              = "unicast-dry-run"
	LockoutDurationKey     = "unicast-lockout-duration"
	MessageRateLimitKey    = "unicast-message-rate-limit"
	BandwidthRateLimitKey  = "unicast-bandwidth-rate-limit"
	BandwidthBurstLimitKey = "unicast-bandwidth-burst-limit"
	// resource manager config keys
	MemoryLimitRatioKey          = "libp2p-memory-limit"
	FileDescriptorsRatioKey      = "libp2p-fd-ratio"
	PeerBaseLimitConnsInboundKey = "libp2p-inbound-conns-limit"
	// connection manager
	HighWatermarkKey = "libp2p-connmgr-high"
	LowWatermarkKey  = "libp2p-connmgr-low"
	GracePeriodKey   = "libp2p-connmgr-grace"
	SilencePeriodKey = "libp2p-connmgr-silence"
	// gossipsub
	PeerScoringKey          = "peer-scoring-enabled"
	LocalMeshLogIntervalKey = "gossipsub-local-mesh-logging-interval"
	ScoreTracerIntervalKey  = "gossipsub-score-tracer-interval"
	// gossipsub validation inspector
	GossipSubRPCInspectorNotificationCacheSizeKey                 = "gossipsub-rpc-inspector-notification-cache-size"
	ValidationInspectorNumberOfWorkersKey                         = "gossipsub-rpc-validation-inspector-workers"
	ValidationInspectorInspectMessageQueueCacheSizeKey            = "gossipsub-rpc-validation-inspector-queue-cache-size"
	ValidationInspectorClusterPrefixedTopicsReceivedCacheSizeKey  = "gossipsub-cluster-prefix-tracker-cache-size"
	ValidationInspectorClusterPrefixedTopicsReceivedCacheDecayKey = "gossipsub-cluster-prefix-tracker-cache-decay"
	ValidationInspectorClusterPrefixDiscardThresholdKey           = "gossipsub-rpc-cluster-prefixed-discard-threshold"
	ValidationInspectorGraftLimitsKey                             = "gossipsub-rpc-graft-limits"
	ValidationInspectorPruneLimitsKey                             = "gossipsub-rpc-prune-limits"

	// DiscardThresholdMapKey key used to set the  discard threshold config limit.
	DiscardThresholdMapKey = "discardthreshold"
	// SafetyThresholdMapKey key used to set the safety threshold config limit.
	SafetyThresholdMapKey = "safetythreshold"
	// RateLimitMapKey key used to set the rate limit config limit.
	RateLimitMapKey = "ratelimit"

	// gossipsub metrics inspector
	MetricsInspectorNumberOfWorkersKey = "gossipsub-rpc-metrics-inspector-workers"
	MetricsInspectorCacheSizeKey       = "gossipsub-rpc-metrics-inspector-cache-size"
)

// toMapStrInt converts map[string]interface{} -> map[string]int
func toMapStrInt(vals map[string]interface{}) map[string]int {
	m := make(map[string]int)
	for key, val := range vals {
		m[key] = val.(int)
	}
	return m
}
