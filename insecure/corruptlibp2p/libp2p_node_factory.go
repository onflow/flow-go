package corruptlibp2p

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	corrupt "github.com/yhassanzadeh13/go-libp2p-pubsub"

	"github.com/onflow/flow-go/network/p2p"

	madns "github.com/multiformats/go-multiaddr-dns"
	"github.com/rs/zerolog"

	fcrypto "github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network/p2p/p2pbuilder"
)

// NewCorruptLibP2PNodeFactory wrapper around the original DefaultLibP2PNodeFactory. Nodes returned from this factory func will be corrupted libp2p nodes.
func NewCorruptLibP2PNodeFactory(
	log zerolog.Logger,
	chainID flow.ChainID,
	address string,
	flowKey fcrypto.PrivateKey,
	sporkId flow.Identifier,
	idProvider module.IdentityProvider,
	metrics module.NetworkMetrics,
	resolver madns.BasicResolver,
	peerScoringEnabled bool,
	role string,
	onInterceptPeerDialFilters,
	onInterceptSecuredFilters []p2p.PeerFilter,
	connectionPruning bool,
	updateInterval time.Duration,
	topicValidatorDisabled bool,
) p2pbuilder.LibP2PFactoryFunc {
	return func() (p2p.LibP2PNode, error) {
		if chainID != flow.BftTestnet {
			panic("illegal chain id for using corruptible conduit factory")
		}

		builder := p2pbuilder.DefaultNodeBuilder(
			log,
			address,
			flowKey,
			sporkId,
			idProvider,
			metrics,
			resolver,
			role,
			onInterceptPeerDialFilters,
			onInterceptSecuredFilters,
			peerScoringEnabled,
			connectionPruning,
			updateInterval)
		if topicValidatorDisabled {
			builder.SetCreateNode(NewCorruptLibP2PNode)
		}
		overrideWithCorruptGossipSub(builder)
		return builder.Build()
	}
}

// CorruptibleGossipSubFactory returns a factory function that creates a new instance of the forked gossipsub module from
// github.com/yhassanzadeh13/go-libp2p-pubsub for the purpose of BFT testing and attack vector implementation.
func CorruptibleGossipSubFactory(routerOpts ...func(*corrupt.GossipSubRouter)) p2pbuilder.GossipSubFactoryFunc {
	factory := func(ctx context.Context, logger zerolog.Logger, host host.Host, cfg p2p.PubSubAdapterConfig) (p2p.PubSubAdapter, error) {
		adapter, router, err := NewCorruptGossipSubAdapter(ctx, logger, host, cfg)
		for _, opt := range routerOpts {
			opt(router)
		}
		return adapter, err
	}
	return factory
}

// CorruptibleGossipSubConfigFactory returns a factory function that creates a new instance of the forked gossipsub config
// from github.com/yhassanzadeh13/go-libp2p-pubsub for the purpose of BFT testing and attack vector implementation.
func CorruptibleGossipSubConfigFactory() p2pbuilder.GossipSubAdapterConfigFunc {
	return func(base *p2p.BasePubSubAdapterConfig) p2p.PubSubAdapterConfig {
		return NewCorruptPubSubAdapterConfig(base)
	}
}

// CorruptibleGossipSubConfigFactoryWithInspector returns a factory function that creates a new instance of the forked gossipsub config
// from github.com/yhassanzadeh13/go-libp2p-pubsub for the purpose of BFT testing and attack vector implementation.
func CorruptibleGossipSubConfigFactoryWithInspector(inspector func(peer.ID, *corrupt.RPC) error) p2pbuilder.GossipSubAdapterConfigFunc {
	return func(base *p2p.BasePubSubAdapterConfig) p2p.PubSubAdapterConfig {
		return NewCorruptPubSubAdapterConfigWithInspector(base, inspector)
	}
}

func overrideWithCorruptGossipSub(builder p2pbuilder.NodeBuilder) {
	factory := CorruptibleGossipSubFactory()
	builder.SetGossipSubFactory(factory, CorruptibleGossipSubConfigFactory())
}
