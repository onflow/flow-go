package validation

const (
	// HardThresholdMapKey key used to set the  hard threshold config limit.
	HardThresholdMapKey = "hardthreshold"
	// SafetyThresholdMapKey key used to set the safety threshold config limit.
	SafetyThresholdMapKey = "safetythreshold"
	// RateLimitMapKey key used to set the rate limit config limit.
	RateLimitMapKey = "ratelimit"
	// DefaultGraftHardThreshold upper bound for graft messages, if the RPC control message GRAFTs exceed this threshold  the RPC control message automatically discarded.
	DefaultGraftHardThreshold = 30
	// DefaultGraftSafetyThreshold a lower bound for graft messages, if the amount of GRAFTs in an RPC control message is below this threshold those GRAFTs validation will be bypassed.
	DefaultGraftSafetyThreshold = .5 * DefaultGraftHardThreshold
	// DefaultGraftRateLimit the rate limit for graft control messages.
	// Currently, the default rate limit is equal to the hard threshold amount.
	// This will result in a rate limit of 30 grafts/sec.
	DefaultGraftRateLimit = DefaultGraftHardThreshold

	// DefaultPruneHardThreshold upper bound for prune messages, if the RPC control message PRUNEs exceed this threshold  the RPC control message automatically discarded.
	DefaultPruneHardThreshold = 30
	// DefaultPruneSafetyThreshold a lower bound for prune messages, if the amount of PRUNEs in an RPC control message is below this threshold those GRAFTs validation will be bypassed.
	DefaultPruneSafetyThreshold = .5 * DefaultPruneHardThreshold

	// DefaultClusterPrefixedMsgDropThreshold is the maximum number of cluster-prefixed control messages allowed to be processed
	// when the cluster IDs provider has not been set or a node is behind in the protocol state. If the number of cluster-prefixed
	// control messages in an RPC exceeds this threshold, the entire RPC will be dropped and the node should be penalized.
	DefaultClusterPrefixedMsgDropThreshold = 100
	// DefaultPruneRateLimit the rate limit for prune control messages.
	// Currently, the default rate limit is equal to the hard threshold amount.
	// This will result in a rate limit of 30 prunes/sec.
	DefaultPruneRateLimit = DefaultPruneHardThreshold

	// DefaultIHaveHardThreshold upper bound for ihave messages, the message count for ihave messages
	// exceeds the configured hard threshold only a sample size of the messages will be inspected. This
	// ensures liveness of the network because there is no expected max number of ihave messages than can be
	// received by a node.
	DefaultIHaveHardThreshold = 100
	// DefaultIHaveSafetyThreshold a lower bound for ihave messages, if the amount of iHaves in an RPC control message is below this threshold those GRAFTs validation will be bypassed.
	DefaultIHaveSafetyThreshold = .5 * DefaultIHaveHardThreshold
	// DefaultIHaveRateLimit rate limiting for ihave control messages is disabled.
	DefaultIHaveRateLimit = 0
	// DefaultIHaveSyncInspectSampleSizePercentage the default percentage of ihaves to use as the sample size for synchronous inspection 25%.
	DefaultIHaveSyncInspectSampleSizePercentage = .25
	// DefaultIHaveAsyncInspectSampleSizePercentage the default percentage of ihaves to use as the sample size for asynchronous inspection 10%.
	DefaultIHaveAsyncInspectSampleSizePercentage = .10
	// DefaultIHaveInspectionMaxSampleSize the max number of ihave messages in a sample to be inspected.
	DefaultIHaveInspectionMaxSampleSize = 100
)
