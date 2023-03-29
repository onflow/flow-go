package p2pbuilder

import (
	"time"

	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/inspector/validation"
)

// UnicastConfig configuration parameters for the unicast manager.
type UnicastConfig struct {
	// StreamRetryInterval is the initial delay between failing to establish a connection with another node and retrying. This
	// delay increases exponentially (exponential backoff) with the number of subsequent failures to establish a connection.
	StreamRetryInterval time.Duration
	// RateLimiterDistributor distributor that distributes notifications whenever a peer is rate limited to all consumers.
	RateLimiterDistributor p2p.UnicastRateLimiterDistributor
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
}

// GossipSubRPCValidationConfigs validation limits used for gossipsub RPC control message inspection.
type GossipSubRPCValidationConfigs struct {
	NumberOfWorkers int
	// GraftLimits GRAFT control message validation limits.
	GraftLimits map[string]int
	// PruneLimits PRUNE control message validation limits.
	PruneLimits map[string]int
	// IHaveLimitsConfig IHAVE control message validation limits configuration.
	IHaveLimitsConfig *GossipSubCtrlMsgIhaveLimitsConfig
}

// GossipSubCtrlMsgIhaveLimitsConfig validation limit configs for ihave RPC control messages.
type GossipSubCtrlMsgIhaveLimitsConfig struct {
	// IHaveLimits IHAVE control message validation limits.
	IHaveLimits map[string]int
	// IHaveSyncInspectSampleSizePercentage the percentage of topics to sample for sync pre-processing in float64 form.
	IHaveSyncInspectSampleSizePercentage float64
	// IHaveAsyncInspectSampleSizePercentage  the percentage of topics to sample for async pre-processing in float64 form.
	IHaveAsyncInspectSampleSizePercentage float64
	// IHaveInspectionMaxSampleSize the max number of ihave messages in a sample to be inspected.
	IHaveInspectionMaxSampleSize float64
}

// IhaveConfigurationOpts returns list of options for the ihave configuration.
func (g *GossipSubCtrlMsgIhaveLimitsConfig) IhaveConfigurationOpts() []validation.CtrlMsgValidationConfigOption {
	return []validation.CtrlMsgValidationConfigOption{
		validation.WithIHaveSyncInspectSampleSizePercentage(g.IHaveSyncInspectSampleSizePercentage),
		validation.WithIHaveAsyncInspectSampleSizePercentage(g.IHaveAsyncInspectSampleSizePercentage),
		validation.WithIHaveInspectionMaxSampleSize(g.IHaveInspectionMaxSampleSize),
	}
}
