package p2p

import (
	"context"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	madns "github.com/multiformats/go-multiaddr-dns"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	flownet "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p/p2pconf"
)

type GossipSubFactoryFunc func(context.Context, zerolog.Logger, host.Host, PubSubAdapterConfig, CollectionClusterChangesConsumer) (PubSubAdapter, error)
type CreateNodeFunc func(zerolog.Logger, host.Host, ProtocolPeerCache, PeerManager, *DisallowListCacheConfig) LibP2PNode
type GossipSubAdapterConfigFunc func(*BasePubSubAdapterConfig) PubSubAdapterConfig

// GossipSubBuilder provides a builder pattern for creating a GossipSub pubsub system.
type GossipSubBuilder interface {
	// SetHost sets the host of the builder.
	// If the host has already been set, a fatal error is logged.
	SetHost(host.Host)

	// SetSubscriptionFilter sets the subscription filter of the builder.
	// If the subscription filter has already been set, a fatal error is logged.
	SetSubscriptionFilter(pubsub.SubscriptionFilter)

	// SetGossipSubFactory sets the gossipsub factory of the builder.
	// We expect the node to initialize with a default gossipsub factory. Hence, this function overrides the default config.
	SetGossipSubFactory(GossipSubFactoryFunc)

	// SetGossipSubConfigFunc sets the gossipsub config function of the builder.
	// We expect the node to initialize with a default gossipsub config. Hence, this function overrides the default config.
	SetGossipSubConfigFunc(GossipSubAdapterConfigFunc)

	// EnableGossipSubScoringWithOverride enables peer scoring for the GossipSub pubsub system with the given override.
	// Any existing peer scoring config attribute that is set in the override will override the default peer scoring config.
	// Anything that is left to nil or zero value in the override will be ignored and the default value will be used.
	// Note: it is not recommended to override the default peer scoring config in production unless you know what you are doing.
	// Production Tip: use PeerScoringConfigNoOverride as the argument to this function to enable peer scoring without any override.
	// Args:
	// - PeerScoringConfigOverride: override for the peer scoring config- Recommended to use PeerScoringConfigNoOverride for production.
	// Returns:
	// none
	EnableGossipSubScoringWithOverride(*PeerScoringConfigOverride)

	// SetGossipSubScoreTracerInterval sets the gossipsub score tracer interval of the builder.
	// If the gossipsub score tracer interval has already been set, a fatal error is logged.
	SetGossipSubScoreTracerInterval(time.Duration)

	// SetGossipSubTracer sets the gossipsub tracer of the builder.
	// If the gossipsub tracer has already been set, a fatal error is logged.
	SetGossipSubTracer(PubSubTracer)

	// SetRoutingSystem sets the routing system of the builder.
	// If the routing system has already been set, a fatal error is logged.
	SetRoutingSystem(routing.Routing)

	// OverrideDefaultRpcInspectorSuiteFactory overrides the default RPC inspector suite factory of the builder.
	// A default RPC inspector suite factory is provided by the node. This function overrides the default factory.
	// The purpose of override is to allow the node to provide a custom RPC inspector suite factory for sake of testing
	// or experimentation.
	// It is NOT recommended to override the default RPC inspector suite factory in production unless you know what you are doing.
	OverrideDefaultRpcInspectorSuiteFactory(GossipSubRpcInspectorSuiteFactoryFunc)

	// Build creates a new GossipSub pubsub system.
	// It returns the newly created GossipSub pubsub system and any errors encountered during its creation.
	//
	// Arguments:
	// - context.Context: the irrecoverable context of the node.
	//
	// Returns:
	// - PubSubAdapter: a GossipSub pubsub system for the libp2p node.
	// - error: if an error occurs during the creation of the GossipSub pubsub system, it is returned. Otherwise, nil is returned.
	// Note that on happy path, the returned error is nil. Any error returned is unexpected and should be handled as irrecoverable.
	Build(irrecoverable.SignalerContext) (PubSubAdapter, error)
}

// GossipSubRpcInspectorSuiteFactoryFunc is a function that creates a new RPC inspector suite. It is used to create
// RPC inspectors for the gossipsub protocol. The RPC inspectors are used to inspect and validate
// incoming RPC messages before they are processed by the gossipsub protocol.
// Args:
// - logger: logger to use
// - sporkID: spork ID of the node
// - cfg: configuration for the RPC inspectors
// - metrics: metrics to use for the RPC inspectors
// - heroCacheMetricsFactory: metrics factory for the hero cache
// - networkingType: networking type of the node, i.e., public or private
// - identityProvider: identity provider of the node
// Returns:
// - p2p.GossipSubInspectorSuite: new RPC inspector suite
// - error: error if any, any returned error is irrecoverable.
type GossipSubRpcInspectorSuiteFactoryFunc func(
	zerolog.Logger,
	flow.Identifier,
	*p2pconf.GossipSubRPCInspectorsConfig,
	module.GossipSubMetrics,
	metrics.HeroCacheMetricsFactory,
	flownet.NetworkingType,
	module.IdentityProvider) (GossipSubInspectorSuite, error)

// NodeBuilder is a builder pattern for creating a libp2p Node instance.
type NodeBuilder interface {
	SetBasicResolver(madns.BasicResolver) NodeBuilder
	SetSubscriptionFilter(pubsub.SubscriptionFilter) NodeBuilder
	SetResourceManager(network.ResourceManager) NodeBuilder
	SetConnectionManager(connmgr.ConnManager) NodeBuilder
	SetConnectionGater(ConnectionGater) NodeBuilder
	SetRoutingSystem(func(context.Context, host.Host) (routing.Routing, error)) NodeBuilder

	// EnableGossipSubScoringWithOverride enables peer scoring for the GossipSub pubsub system with the given override.
	// Any existing peer scoring config attribute that is set in the override will override the default peer scoring config.
	// Anything that is left to nil or zero value in the override will be ignored and the default value will be used.
	// Note: it is not recommended to override the default peer scoring config in production unless you know what you are doing.
	// Production Tip: use PeerScoringConfigNoOverride as the argument to this function to enable peer scoring without any override.
	// Args:
	// - PeerScoringConfigOverride: override for the peer scoring config- Recommended to use PeerScoringConfigNoOverride for production.
	// Returns:
	// none
	EnableGossipSubScoringWithOverride(*PeerScoringConfigOverride) NodeBuilder
	SetCreateNode(CreateNodeFunc) NodeBuilder
	SetGossipSubFactory(GossipSubFactoryFunc, GossipSubAdapterConfigFunc) NodeBuilder
	SetStreamCreationRetryInterval(time.Duration) NodeBuilder
	SetRateLimiterDistributor(UnicastRateLimiterDistributor) NodeBuilder
	SetGossipSubTracer(PubSubTracer) NodeBuilder
	SetGossipSubScoreTracerInterval(time.Duration) NodeBuilder
	OverrideDefaultRpcInspectorSuiteFactory(GossipSubRpcInspectorSuiteFactoryFunc) NodeBuilder
	Build() (LibP2PNode, error)
}

// PeerScoringConfigOverride is a structure that is used to carry over the override values for peer scoring configuration.
// Any attribute that is set in the override will override the default peer scoring config.
// Typically, we are not recommending to override the default peer scoring config in production unless you know what you are doing.
type PeerScoringConfigOverride struct {
	// TopicScoreParams is a map of topic score parameters for each topic.
	// Override criteria: any topic (i.e., key in the map) will override the default topic score parameters for that topic and
	// the corresponding value in the map will be used instead of the default value.
	// If you don't want to override topic score params for a given topic, simply don't include that topic in the map.
	// If the map is nil, the default topic score parameters are used for all topics.
	TopicScoreParams map[channels.Topic]*pubsub.TopicScoreParams

	// AppSpecificScoreParams is a function that returns the application specific score parameters for a given peer.
	// Override criteria: if the function is not nil, it will override the default application specific score parameters.
	// If the function is nil, the default application specific score parameters are used.
	AppSpecificScoreParams func(peer.ID) float64

	// DecayInterval is the interval over which we decay the effect of past behavior, so that
	// a good or bad behavior will not have a permanent effect on the penalty. It is also the interval
	// that GossipSub uses to refresh the scores of all peers.
	// Override criteria: if the value is not zero, it will override the default decay interval.
	// If the value is zero, the default decay interval is used.
	DecayInterval time.Duration
}

// PeerScoringConfigNoOverride is a default peer scoring configuration for a GossipSub pubsub system.
// It is set to nil, which means that no override is done to the default peer scoring configuration.
// It is the recommended way to use the default peer scoring configuration.
var PeerScoringConfigNoOverride = (*PeerScoringConfigOverride)(nil)
