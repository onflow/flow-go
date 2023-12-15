package netconf

import (
	"time"
)

const (
	gossipsubKey         = "gossipsub"
	unicastKey           = "unicast"
	connectionManagerKey = "connection-manager"
)

// Config encapsulation of configuration structs for all components related to the Flow network.
type Config struct {
	Unicast           Unicast                      `mapstructure:"unicast"`
	ResourceManager   config.ResourceManagerConfig `mapstructure:"libp2p-resource-manager"`
	ConnectionManager ConnectionManager            `mapstructure:"connection-manager"`
	// GossipSub core gossipsub configuration.
	GossipSub  config.GossipSubParameters `mapstructure:"gossipsub"`
	AlspConfig `mapstructure:",squash"`

	// NetworkConnectionPruning determines whether connections to nodes
	// that are not part of protocol state should be trimmed
	// TODO: solely a fallback mechanism, can be removed upon reliable behavior in production.
	NetworkConnectionPruning bool `mapstructure:"networking-connection-pruning"`
	// PreferredUnicastProtocols list of unicast protocols in preferred order
	PreferredUnicastProtocols       []string      `mapstructure:"preferred-unicast-protocols"`
	NetworkReceivedMessageCacheSize uint32        `validate:"gt=0" mapstructure:"received-message-cache-size"`
	PeerUpdateInterval              time.Duration `validate:"gt=0s" mapstructure:"peerupdate-interval"`

	DNSCacheTTL time.Duration `validate:"gt=0s" mapstructure:"dns-cache-ttl"`
	// DisallowListNotificationCacheSize size of the queue for notifications about new peers in the disallow list.
	DisallowListNotificationCacheSize uint32 `validate:"gt=0" mapstructure:"disallow-list-notification-cache-size"`
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

	SyncEngine SyncEngineAlspConfig `mapstructure:",squash"`
}

// SyncEngineAlspConfig is the ALSP config for the SyncEngine.
type SyncEngineAlspConfig struct {
	// BatchRequestBaseProb is the base probability in [0,1] that's used in creating the final probability of creating a
	// misbehavior report for a BatchRequest message. This is why the word "base" is used in the name of this field,
	// since it's not the final probability and there are other factors that determine the final probability.
	// The reason for this is that we want to increase the probability of creating a misbehavior report for a large batch.
	BatchRequestBaseProb float32 `validate:"gte=0,lte=1" mapstructure:"alsp-sync-engine-batch-request-base-prob"`

	// RangeRequestBaseProb is the base probability in [0,1] that's used in creating the final probability of creating a
	// misbehavior report for a RangeRequest message. This is why the word "base" is used in the name of this field,
	// since it's not the final probability and there are other factors that determine the final probability.
	// The reason for this is that we want to increase the probability of creating a misbehavior report for a large range.
	RangeRequestBaseProb float32 `validate:"gte=0,lte=1" mapstructure:"alsp-sync-engine-range-request-base-prob"`

	// SyncRequestProb is the probability in [0,1] of creating a misbehavior report for a SyncRequest message.
	SyncRequestProb float32 `validate:"gte=0,lte=1" mapstructure:"alsp-sync-engine-sync-request-prob"`
}
