package internal

import (
	"github.com/onflow/flow-go/network/p2p/p2pbuilder"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
)

func WithCorruptGossipSub(factory p2pbuilder.GossipSubFactoryFunc, config p2pbuilder.GossipSubAdapterConfigFunc) p2ptest.NodeFixtureParameterOption {
	return func(p *p2ptest.NodeFixtureParameters) {
		p.GossipSubFactory = factory
		p.GossipSubConfig = config
	}
}
