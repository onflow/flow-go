package p2pbuilder

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/queue"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/distributor"
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
}

// HeroStoreOpts returns hero store options.
func HeroStoreOpts(cacheSize uint32, metricsCollector *metrics.HeroCacheCollector) []queue.HeroStoreConfigOption {
	heroStoreOpts := []queue.HeroStoreConfigOption{queue.WithHeroStoreSizeLimit(cacheSize)}
	if metricsCollector != nil {
		heroStoreOpts = append(heroStoreOpts, queue.WithHeroStoreCollector(metricsCollector))
	}
	return heroStoreOpts
}

// GossipSubRPCInspector helper that sets up the gossipsub RPC validation inspector and notification distributor.
func GossipSubRPCInspector(logger zerolog.Logger,
	sporkId flow.Identifier,
	validationConfigs *GossipSubRPCValidationConfigs,
	heroStoreOpts ...queue.HeroStoreConfigOption,
) (*validation.ControlMsgValidationInspector, *distributor.GossipSubInspectorNotificationDistributor, error) {
	controlMsgRPCInspectorCfg, err := gossipSubRPCInspectorConfig(validationConfigs, heroStoreOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create gossipsub rpc inspector config: %w", err)
	}
	gossipSubInspectorNotifDistributor := distributor.DefaultGossipSubInspectorNotificationDistributor(logger)
	rpcValidationInspector := validation.NewControlMsgValidationInspector(logger, sporkId, controlMsgRPCInspectorCfg, gossipSubInspectorNotifDistributor)
	return rpcValidationInspector, gossipSubInspectorNotifDistributor, nil
}

// gossipSubRPCInspectorConfig returns a new inspector.ControlMsgValidationInspectorConfig using configuration provided by the node builder.
func gossipSubRPCInspectorConfig(validationConfigs *GossipSubRPCValidationConfigs, opts ...queue.HeroStoreConfigOption) (*validation.ControlMsgValidationInspectorConfig, error) {
	// setup rpc validation configuration for each control message type
	graftValidationCfg, err := validation.NewCtrlMsgValidationConfig(p2p.CtrlMsgGraft, validationConfigs.GraftLimits)
	if err != nil {
		return nil, fmt.Errorf("failed to create gossupsub RPC validation configuration: %w", err)
	}
	pruneValidationCfg, err := validation.NewCtrlMsgValidationConfig(p2p.CtrlMsgPrune, validationConfigs.PruneLimits)
	if err != nil {
		return nil, fmt.Errorf("failed to create gossupsub RPC validation configuration: %w", err)
	}

	// setup gossip sub RPC control message inspector config
	controlMsgRPCInspectorCfg := &validation.ControlMsgValidationInspectorConfig{
		NumberOfWorkers:     validationConfigs.NumberOfWorkers,
		InspectMsgStoreOpts: opts,
		GraftValidationCfg:  graftValidationCfg,
		PruneValidationCfg:  pruneValidationCfg,
	}
	return controlMsgRPCInspectorCfg, nil
}
