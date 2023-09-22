package p2pconfig

import (
	"time"

	"github.com/onflow/flow-go/network/p2p"
)

// UnicastConfig configuration parameters for the unicast manager.
type UnicastConfig struct {
	// StreamRetryInterval is the initial delay between failing to establish a connection with another node and retrying. This
	// delay increases exponentially (exponential backoff) with the number of subsequent failures to establish a connection.
	StreamRetryInterval time.Duration

	// TODO: removing this field allows to directly use the netconf/config UnicastConfig struct for the rest.
	// RateLimiterDistributor distributor that distributes notifications whenever a peer is rate limited to all consumers.
	RateLimiterDistributor p2p.UnicastRateLimiterDistributor

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
	StreamZeroRetryResetThreshold uint64

	// DialZeroRetryResetThreshold is the threshold that determines when to reset the dial retry budget to the default value.
	// For example the threshold of 1 hour means that if the dial retry budget is decreased to 0, then it will be reset to default value
	// when it has been 1 hour since the last successful dial.
	//
	// This is to prevent the retry budget from being reset too frequently, as the retry budget is used to gauge the reliability of the dialing a remote peer.
	// When the dial retry budget is reset to the default value, it means that the dialing is reliable enough to be trusted again.
	// This parameter mandates when the dialing is reliable enough to be trusted again; i.e., when it has been 1 hour since the last successful dial.
	// Note that the last dial attempt timestamp is reset to zero when the dial fails, so the value of for example 1 hour means that the dialing to the remote peer is reliable enough that the last
	// successful dial attempt was 1 hour ago.
	DialZeroRetryResetThreshold time.Duration

	// MaxDialRetryAttemptTimes is the maximum number of attempts to be made to connect to a remote node to establish a unicast (1:1) connection before we give up.
	MaxDialRetryAttemptTimes uint64

	// MaxStreamCreationRetryAttemptTimes is the maximum number of attempts to be made to create a stream to a remote node over a direct unicast (1:1) connection before we give up.
	MaxStreamCreationRetryAttemptTimes uint64
}

// ConnectionGaterConfig configuration parameters for the connection gater.
type ConnectionGaterConfig struct {
	// InterceptPeerDialFilters list of peer filters used to filter peers on outgoing connections in the InterceptPeerDial callback.
	InterceptPeerDialFilters []p2p.PeerFilter
	// InterceptSecuredFilters list of peer filters used to filter peers and accept or reject inbound connections in InterceptSecured callback.
	InterceptSecuredFilters []p2p.PeerFilter
}

// PeerManagerConfig configuration parameters for the peer manager.
type PeerManagerConfig struct {
	// ConnectionPruning enables connection pruning in the connection manager.
	ConnectionPruning bool
	// UpdateInterval interval used by the libp2p node peer manager component to periodically request peer updates.
	UpdateInterval time.Duration
	// ConnectorFactory is a factory function to create a new connector.
	ConnectorFactory p2p.ConnectorFactory
}

// PeerManagerDisableConfig returns a configuration that disables the peer manager.
func PeerManagerDisableConfig() *PeerManagerConfig {
	return &PeerManagerConfig{
		ConnectionPruning: false,
		UpdateInterval:    0,
		ConnectorFactory:  nil,
	}
}
