package p2p

import (
	"context"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/multiformats/go-multiaddr-dns"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/p2p/scoring"
)

// LibP2PFactoryFunc is a factory function type for generating libp2p Node instances.
type LibP2PFactoryFunc func() (LibP2PNode, error)
type GossipSubFactoryFunc func(context.Context, zerolog.Logger, host.Host, PubSubAdapterConfig) (PubSubAdapter, error)
type CreateNodeFunc func(zerolog.Logger, host.Host, ProtocolPeerCache, PeerManager) LibP2PNode
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
	// If the gossipsub factory has already been set, a fatal error is logged.
	SetGossipSubFactory(GossipSubFactoryFunc)

	// SetGossipSubConfigFunc sets the gossipsub config function of the builder.
	// If the gossipsub config function has already been set, a fatal error is logged.
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

	// SetIDProvider sets the identity provider of the builder.
	// If the identity provider has already been set, a fatal error is logged.
	SetIDProvider(module.IdentityProvider)

	// SetRoutingSystem sets the routing system of the builder.
	// If the routing system has already been set, a fatal error is logged.
	SetRoutingSystem(routing.Routing)

	// SetPeerScoringParameterOptions sets the peer scoring parameter options of the builder.
	// If the peer scoring parameter options have already been set, a fatal error is logged.
	SetPeerScoringParameterOptions(options ...scoring.PeerScoreParamsOption)

	// Build creates a new GossipSub pubsub system.
	// It returns the newly created GossipSub pubsub system and any errors encountered during its creation.
	//
	// Arguments:
	// - context.Context: the irrecoverable context of the node.
	//
	// Returns:
	// - PubSubAdapter: a GossipSub pubsub system for the libp2p node.
	// - PeerScoreTracer: a peer score tracer for the GossipSub pubsub system (if enabled, otherwise nil).
	// - error: if an error occurs during the creation of the GossipSub pubsub system, it is returned. Otherwise, nil is returned.
	// Note that on happy path, the returned error is nil. Any error returned is unexpected and should be handled as irrecoverable.
	Build(irrecoverable.SignalerContext) (PubSubAdapter, PeerScoreTracer, error)
}

// NodeBuilder is a builder pattern for creating a libp2p Node instance.
type NodeBuilder interface {
	SetBasicResolver(madns.BasicResolver) NodeBuilder
	SetSubscriptionFilter(pubsub.SubscriptionFilter) NodeBuilder
	SetResourceManager(network.ResourceManager) NodeBuilder
	SetConnectionManager(connmgr.ConnManager) NodeBuilder
	SetConnectionGater(connmgr.ConnectionGater) NodeBuilder
	SetRoutingSystem(func(context.Context, host.Host) (routing.Routing, error)) NodeBuilder
	SetPeerManagerOptions(connectionPruning bool, updateInterval time.Duration) NodeBuilder
	EnableGossipSubPeerScoring(provider module.IdentityProvider, ops ...scoring.PeerScoreParamsOption) NodeBuilder
	SetCreateNode(CreateNodeFunc) NodeBuilder
	SetGossipSubFactory(GossipSubFactoryFunc, GossipSubAdapterConfigFunc) NodeBuilder
	SetStreamCreationRetryInterval(createStreamRetryInterval time.Duration) NodeBuilder
	SetRateLimiterDistributor(consumer UnicastRateLimiterDistributor) NodeBuilder
	SetGossipSubTracer(tracer PubSubTracer) NodeBuilder
	SetGossipSubScoreTracerInterval(interval time.Duration) NodeBuilder
	Build() (LibP2PNode, error)
}
