package netconf

import "time"

const (
	RateLimiterKey            = "rate-limiter"
	unicastManagerKey         = "manager"
	EnableStreamProtectionKey = "enable-stream-protection"
	MessageTimeoutKey         = "message-timeout"
)

// Unicast configuration parameters for the unicast protocol.
type Unicast struct {
	// RateLimiter configuration for all unicast rate limiters.
	RateLimiter RateLimiter `mapstructure:"rate-limiter"`

	// UnicastManager configuration for the unicast manager. The unicast manager is responsible for establishing unicast streams.
	UnicastManager UnicastManager `mapstructure:"manager"`

	// EnableStreamProtection enables stream protection for unicast streams. When enabled, all connections that are being established or
	// have been already established for unicast streams are protected, meaning that they won't be closed by the connection manager.
	// This is useful for preventing the connection manager from closing unicast streams that are being used by the application layer.
	// However, it may interfere with the resource manager of libp2p, i.e., the connection manager may not be able to close connections
	// that are not being used by the application layer while at the same time the node is running out of resources for new connections.
	EnableStreamProtection bool `mapstructure:"enable-stream-protection"`

	MessageTimeout time.Duration `validate:"gt=0s" mapstructure:"message-timeout"`
}

const (
	DryRunKey              = "dry-run"
	LockoutDurationKey     = "lockout-duration"
	messageRateLimitKey    = "message-rate-limit"
	BandwidthRateLimitKey  = "bandwidth-rate-limit"
	BandwidthBurstLimitKey = "bandwidth-burst-limit"
)

// RateLimiter unicast rate limiter configuration for the message and bandwidth rate limiters.
type RateLimiter struct {
	// DryRun setting this to true will disable connection disconnects and gating when unicast rate limiters are configured
	DryRun bool `mapstructure:"dry-run"`
	// LockoutDuration the number of seconds a peer will be forced to wait before being allowed to successfully reconnect to the node
	// after being rate limited.
	LockoutDuration time.Duration `validate:"gte=0" mapstructure:"lockout-duration"`
	// MessageRateLimit amount of unicast messages that can be sent by a peer per second.
	MessageRateLimit int `validate:"gte=0" mapstructure:"message-rate-limit"`
	// BandwidthRateLimit bandwidth size in bytes a peer is allowed to send via unicast streams per second.
	BandwidthRateLimit int `validate:"gte=0" mapstructure:"bandwidth-rate-limit"`
	// BandwidthBurstLimit bandwidth size in bytes a peer is allowed to send via unicast streams at once.
	BandwidthBurstLimit int `validate:"gte=0" mapstructure:"bandwidth-burst-limit"`
}

const (
	createStreamBackoffDelayKey           = "create-stream-retry-delay"
	streamZeroRetryResetThresholdKey      = "stream-zero-retry-reset-threshold"
	maxStreamCreationRetryAttemptTimesKey = "max-stream-creation-retry-attempt-times"
	configCacheSizeKey                    = "dial-config-cache-size"
)

// UnicastManager configuration for the unicast manager. The unicast manager is responsible for establishing unicast streams.
type UnicastManager struct {
	// CreateStreamBackoffDelay initial delay used in the exponential backoff for create stream retries.
	CreateStreamBackoffDelay time.Duration `validate:"gt=0s" mapstructure:"create-stream-retry-delay"`
	// StreamZeroRetryResetThreshold is the threshold that determines when to reset the stream creation retry budget to the default value.
	//
	// For example the default value of 100 means that if the stream creation retry budget is decreased to 0, then it will be reset to default value
	// when the number of consecutive successful streams reaches 100.
	//
	// This is to prevent the retry budget from being reset too frequently, as the retry budget is used to gauge the reliability of the stream creation.
	// When the stream creation retry budget is reset to the default value, it means that the stream creation is reliable enough to be trusted again.
	// This parameter mandates when the stream creation is reliable enough to be trusted again; i.e., when the number of consecutive successful streams reaches this threshold.
	// Note that the counter is reset to 0 when the stream creation fails, so the value of for example 100 means that the stream creation is reliable enough that the recent
	// 100 stream creations are all successful.
	StreamZeroRetryResetThreshold uint64 `validate:"gt=0" mapstructure:"stream-zero-retry-reset-threshold"`
	// MaxStreamCreationRetryAttemptTimes is the maximum number of attempts to be made to create a stream to a remote node over a direct unicast (1:1) connection before we give up.
	MaxStreamCreationRetryAttemptTimes uint64 `validate:"gt=1" mapstructure:"max-stream-creation-retry-attempt-times"`
	// ConfigCacheSize is the cache size of the dial config cache that keeps the individual dial config for each peer.
	ConfigCacheSize uint32 `validate:"gt=0" mapstructure:"dial-config-cache-size"`
}
