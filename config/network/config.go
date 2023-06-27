package network

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config encapsulation of configuration structs for all components related to the Flow network.
type Config struct {
	// UnicastRateLimitersConfig configuration for all unicast rate limiters.
	UnicastRateLimitersConfig `mapstructure:",squash"`
	ResourceManagerConfig     `mapstructure:",squash"`
	ConnectionManagerConfig   `mapstructure:",squash"`
	// GossipSubConfig core gossipsub configuration.
	GossipSubConfig `mapstructure:",squash"`
	AlspConfig      `mapstructure:",squash"`

	// NetworkConnectionPruning determines whether connections to nodes
	// that are not part of protocol state should be trimmed
	// TODO: solely a fallback mechanism, can be removed upon reliable behavior in production.
	NetworkConnectionPruning bool `mapstructure:"networking-connection-pruning"`
	// PreferredUnicastProtocols list of unicast protocols in preferred order
	PreferredUnicastProtocols       []string      `mapstructure:"preferred-unicast-protocols"`
	NetworkReceivedMessageCacheSize uint32        `validate:"gt=0" mapstructure:"received-message-cache-size"`
	PeerUpdateInterval              time.Duration `validate:"gt=0s" mapstructure:"peerupdate-interval"`
	UnicastMessageTimeout           time.Duration `validate:"gt=0s" mapstructure:"unicast-message-timeout"`
	// UnicastCreateStreamRetryDelay initial delay used in the exponential backoff for create stream retries
	UnicastCreateStreamRetryDelay time.Duration `validate:"gt=0s" mapstructure:"unicast-create-stream-retry-delay"`
	DNSCacheTTL                   time.Duration `validate:"gt=0s" mapstructure:"dns-cache-ttl"`
	// DisallowListNotificationCacheSize size of the queue for notifications about new peers in the disallow list.
	DisallowListNotificationCacheSize uint32 `validate:"gt=0" mapstructure:"disallow-list-notification-cache-size"`
}

// UnicastRateLimitersConfig unicast rate limiter configuration for the message and bandwidth rate limiters.
type UnicastRateLimitersConfig struct {
	// DryRun setting this to true will disable connection disconnects and gating when unicast rate limiters are configured
	DryRun bool `mapstructure:"unicast-dry-run"`
	// LockoutDuration the number of seconds a peer will be forced to wait before being allowed to successfully reconnect to the node
	// after being rate limited.
	LockoutDuration time.Duration `validate:"gte=0" mapstructure:"unicast-lockout-duration"`
	// MessageRateLimit amount of unicast messages that can be sent by a peer per second.
	MessageRateLimit int `validate:"gte=0" mapstructure:"unicast-message-rate-limit"`
	// BandwidthRateLimit bandwidth size in bytes a peer is allowed to send via unicast streams per second.
	BandwidthRateLimit int `validate:"gte=0" mapstructure:"unicast-bandwidth-rate-limit"`
	// BandwidthBurstLimit bandwidth size in bytes a peer is allowed to send via unicast streams at once.
	BandwidthBurstLimit int `validate:"gte=0" mapstructure:"unicast-bandwidth-burst-limit"`
}

// AlspConfig is the config for the Application Layer Spam Prevention (ALSP) protocol.
type AlspConfig struct {
	// Size of the cache for spam records. There is at most one spam record per authorized (i.e., staked) node.
	// Recommended size is 10 * number of authorized nodes to allow for churn.
	SpamRecordCacheSize uint32 `mapstructure:"alsp-spam-record-cache-size"`

	// SpamReportQueueSize is the size of the queue for spam records. The queue is used to store spam records
	// temporarily till they are picked by the workers. When the queue is full, new spam records are dropped.
	// Recommended size is 100 * number of authorized nodes to allow for churn.
	SpamReportQueueSize uint32 `mapstructure:"alsp-spam-report-queue-size"`

	// DisablePenalty indicates whether applying the penalty to the misbehaving node is disabled.
	// When disabled, the ALSP module logs the misbehavior reports and updates the metrics, but does not apply the penalty.
	// This is useful for managing production incidents.
	// Note: under normal circumstances, the ALSP module should not be disabled.
	DisablePenalty bool `mapstructure:"alsp-disable-penalty"`

	// HeartBeatInterval is the interval between heartbeats sent by the ALSP module. The heartbeats are recurring
	// events that are used to perform critical ALSP tasks, such as updating the spam records cache.
	HearBeatInterval time.Duration `mapstructure:"alsp-heart-beat-interval"`
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
