package config

import (
	"time"
)

// NetworkConnectionPruning returns the network connection pruning config value.
func NetworkConnectionPruning() bool {
	return conf.GetBool(NetworkingConnectionPruningKey)
}

// PreferredUnicastsProtocols returns the preferred unicasts protocols config value.
func PreferredUnicastsProtocols() []string {
	return conf.GetStringSlice(NetworkingConnectionPruningKey)
}

// ReceivedMessageCacheSize returns the received message cache size config value.
func ReceivedMessageCacheSize() uint32 {
	return conf.GetUint32(NetworkingConnectionPruningKey)
}

// PeerUpdateInterval returns the peer update interval config value.
func PeerUpdateInterval() time.Duration {
	return conf.GetDuration(NetworkingConnectionPruningKey)
}

// UnicastMessageTimeout returns the unicast message timeout config value.
func UnicastMessageTimeout() time.Duration {
	return conf.GetDuration(NetworkingConnectionPruningKey)
}

// UnicastCreateStreamRetryDelay returns the unicast create stream delay config value.
func UnicastCreateStreamRetryDelay() time.Duration {
	return conf.GetDuration(NetworkingConnectionPruningKey)
}

// DnsCacheTTL returns the network connection pruning config value.
func DnsCacheTTL() time.Duration {
	return conf.GetDuration(NetworkingConnectionPruningKey)
}

// DisallowListNotificationCacheSize returns the network connection pruning config value.
func DisallowListNotificationCacheSize() uint32 {
	return conf.GetUint32(NetworkingConnectionPruningKey)
}

// MessageRateLimit returns the message rate limit config value.
func MessageRateLimit() int {
	return conf.GetInt(MessageRateLimitKey)
}

// BandwidthRateLimit returns the bandwidth rate limit config value.
func BandwidthRateLimit() int {
	return conf.GetInt(BandwidthRateLimitKey)
}

// BandwidthBurstLimit returns the bandwidth burst limit config value.
func BandwidthBurstLimit() int {
	return conf.GetInt(BandwidthBurstLimitKey)
}

// LockoutDuration returns the lockout duration config value.
func LockoutDuration() time.Duration {
	return conf.GetDuration(LockoutDurationKey)
}

// DryRun returns the dry run config value.
func DryRun() bool {
	return conf.GetBool(DryRunKey)
}

// MemoryLimitRatio returns the memory limit ratio config value.
func MemoryLimitRatio() float64 {
	return conf.GetFloat64(MemoryLimitRatioKey)
}

// FileDescriptorsRatio returns the file descriptors ratio config value.
func FileDescriptorsRatio() float64 {
	return conf.GetFloat64(FileDescriptorsRatioKey)
}

// PeerBaseLimitConnsInbound returns the peer base limit connections inbound config value.
func PeerBaseLimitConnsInbound() int {
	return conf.GetInt(PeerBaseLimitConnsInboundKey)
}

// ConnManagerLowWatermark returns the conn manager lower watermark config value.
func ConnManagerLowWatermark() int {
	return conf.GetInt(LowWatermarkKey)
}

// ConnManagerHighWatermark returns the conn manager high watermark config value.
func ConnManagerHighWatermark() int {
	return conf.GetInt(HighWatermarkKey)
}

// ConnManagerGracePeriod returns the conn manager grace period config value.
func ConnManagerGracePeriod() time.Duration {
	return conf.GetDuration(GracePeriodKey)
}

// ConnManagerSilencePeriod returns the conn manager silence period config value.
func ConnManagerSilencePeriod() time.Duration {
	return conf.GetDuration(SilencePeriodKey)
}

// GossipsubPeerScoring returns the gossipsub peer scoring config value.
func GossipsubPeerScoring() bool {
	return conf.GetBool(PeerScoringKey)
}

// GossipsubLocalMeshLogInterval returns the gossipsub local mesh log interval config value.
func GossipsubLocalMeshLogInterval() time.Duration {
	return conf.GetDuration(LocalMeshLogIntervalKey)
}

// GossipsubScoreTracerInterval returns the gossipsub score tracer interval config value.
func GossipsubScoreTracerInterval() time.Duration {
	return conf.GetDuration(ScoreTracerIntervalKey)
}

// ValidationInspectorNumberOfWorkers returns the validation inspector number of workers config value.
func ValidationInspectorNumberOfWorkers() int {
	return conf.GetInt(ValidationInspectorNumberOfWorkersKey)
}

// ValidationInspectorInspectMessageQueueCacheSize returns the validation inspector inspect message queue size config value.
func ValidationInspectorInspectMessageQueueCacheSize() uint32 {
	return conf.GetUint32(ValidationInspectorInspectMessageQueueCacheSizeKey)
}

// ValidationInspectorClusterPrefixedTopicsReceivedCacheSize returns the validation inspector cluster prefixed topics received cache size config value.
func ValidationInspectorClusterPrefixedTopicsReceivedCacheSize() uint32 {
	return conf.GetUint32(ValidationInspectorClusterPrefixedTopicsReceivedCacheSizeKey)
}

// ValidationInspectorClusterPrefixedTopicsReceivedCacheDecay returns the validation inspector cluster prefixed topics received cache decay config value.
func ValidationInspectorClusterPrefixedTopicsReceivedCacheDecay() float64 {
	return conf.GetFloat64(ValidationInspectorClusterPrefixedTopicsReceivedCacheDecayKey)
}

// ValidationInspectorClusterPrefixDiscardThreshold returns the validation inspector cluster prefixed discard threshold config value.
func ValidationInspectorClusterPrefixDiscardThreshold() float64 {
	return conf.GetFloat64(ValidationInspectorClusterPrefixDiscardThresholdKey)
}

// ValidationInspectorGraftLimits returns the validation inspector graft limits config value.
func ValidationInspectorGraftLimits() map[string]int {
	limits := conf.Get(ValidationInspectorGraftLimitsKey).(map[string]interface{})
	return toMapStrInt(limits)
}

// ValidationInspectorPruneLimits returns the validation inspector prune limits config value.
func ValidationInspectorPruneLimits() map[string]int {
	limits := conf.Get(ValidationInspectorPruneLimitsKey).(map[string]interface{})
	return toMapStrInt(limits)
}

// GossipSubRPCInspectorNotificationCacheSize returns the gossipsub rpc inspector notification cache size config value.
func GossipSubRPCInspectorNotificationCacheSize() uint32 {
	return conf.GetUint32(GossipSubRPCInspectorNotificationCacheSizeKey)
}

// MetricsInspectorNumberOfWorkers returns the metrics inspector number of workers config value.
func MetricsInspectorNumberOfWorkers() int {
	return conf.GetInt(MetricsInspectorNumberOfWorkersKey)
}

// MetricsInspectorCacheSize returns the metrics inspector cache size config value.
func MetricsInspectorCacheSize() uint32 {
	return conf.GetUint32(MetricsInspectorCacheSizeKey)
}
