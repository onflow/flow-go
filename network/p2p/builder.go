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
	PeerScoringBuilder
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

	// SetGossipSubPeerScoring sets the gossipsub peer scoring of the builder.
	// If the gossipsub peer scoring flag has already been set, a fatal error is logged.
	SetGossipSubPeerScoring(bool)

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

type PeerScoringBuilder interface {
	// SetTopicScoreParams sets the topic score parameters for the given topic.
	// If the topic score parameters have already been set for the given topic, it is overwritten.
	SetTopicScoreParams(topic channels.Topic, topicScoreParams *pubsub.TopicScoreParams)

	// SetAppSpecificScoreParams sets the application specific score parameters for the given topic.
	// If the application specific score parameters have already been set for the given topic, it is overwritten.
	SetAppSpecificScoreParams(func(peer.ID) float64)
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
	SetPeerManagerOptions(bool, time.Duration) NodeBuilder

	// EnableGossipSubPeerScoring enables peer scoring for the GossipSub pubsub system.
	// Arguments:
	// - module.IdentityProvider: the identity provider for the node (must be set before calling this method).
	// - *PeerScoringConfig: the peer scoring configuration for the GossipSub pubsub system. If nil, the default configuration is used.
	EnableGossipSubPeerScoring(*PeerScoringConfig) NodeBuilder
	SetCreateNode(CreateNodeFunc) NodeBuilder
	SetGossipSubFactory(GossipSubFactoryFunc, GossipSubAdapterConfigFunc) NodeBuilder
	SetStreamCreationRetryInterval(time.Duration) NodeBuilder
	SetRateLimiterDistributor(UnicastRateLimiterDistributor) NodeBuilder
	SetGossipSubTracer(PubSubTracer) NodeBuilder
	SetGossipSubScoreTracerInterval(time.Duration) NodeBuilder
	OverrideDefaultRpcInspectorSuiteFactory(GossipSubRpcInspectorSuiteFactoryFunc) NodeBuilder
	Build() (LibP2PNode, error)
}

// PeerScoringConfig is a configuration for peer scoring parameters for a GossipSub pubsub system.
type PeerScoringConfig struct {
	// TopicScoreParams is a map of topic score parameters for each topic.
	TopicScoreParams map[channels.Topic]*pubsub.TopicScoreParams
	// AppSpecificScoreParams is a function that returns the application specific score parameters for a given peer.
	AppSpecificScoreParams func(peer.ID) float64
}
