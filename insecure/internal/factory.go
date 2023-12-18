package internal

import (
	"github.com/onflow/flow-go/network/p2p"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
)

func WithCorruptGossipSub(factory p2p.GossipSubFactoryFunc, config p2p.GossipSubAdapterConfigFunc) p2ptest.NodeFixtureParameterOption {
	return func(p *p2ptest.NodeFixtureParameters) {
		p.GossipSubFactory = factory
		p.GossipSubConfig = config
	}
}
