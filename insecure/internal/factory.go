package internal

import (
	"context"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/insecure/corruptlibp2p"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/p2pbuilder"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
)

func WithCorruptGossipSub(factory p2pbuilder.GossipSubFactoryFuc, config p2pbuilder.GossipSubAdapterConfigFunc) p2ptest.NodeFixtureParameterOption {
	return func(p *p2ptest.NodeFixtureParameters) {
		p.GossipSubFactory = factory
		p.GossipSubConfig = config
	}
}

// CorruptibleGossipSubFactory returns a factory function that creates a new instance of the forked gossipsub module from
// github.com/yhassanzadeh13/go-libp2p-pubsub for the purpose of BFT testing and attack vector implementation.
func CorruptibleGossipSubFactory() (p2pbuilder.GossipSubFactoryFuc, *CorruptGossipSubRouter) {
	var rt *CorruptGossipSubRouter
	factory := func(ctx context.Context, logger zerolog.Logger, host host.Host, cfg p2p.PubSubAdapterConfig) (p2p.PubSubAdapter, error) {
		adapter, router, err := corruptlibp2p.NewCorruptGossipSubAdapter(ctx, logger, host, cfg)
		rt = router
		return adapter, err
	}
	return factory, rt
}

// CorruptibleGossipSubConfigFactory returns a factory function that creates a new instance of the forked gossipsub config
// from github.com/yhassanzadeh13/go-libp2p-pubsub for the purpose of BFT testing and attack vector implementation.
func CorruptibleGossipSubConfigFactory() p2pbuilder.GossipSubAdapterConfigFunc {
	return func(base *p2p.BasePubSubAdapterConfig) p2p.PubSubAdapterConfig {
		return corruptlibp2p.NewCorruptPubSubAdapterConfig(base)
	}
}

func OverrideWithCorruptGossipSub(builder p2pbuilder.NodeBuilder) {
	factory, _ := CorruptibleGossipSubFactory()
	builder.SetGossipSubFactory(factory, CorruptibleGossipSubConfigFactory())
}
