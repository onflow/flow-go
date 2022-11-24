package p2pnode

import (
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/routing"
	discoveryrouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"

	"github.com/onflow/flow-go/network/p2p"
)

// GossipSubAdapterConfig is a wrapper around libp2p pubsub options that
// implements the PubSubAdapterConfig interface for the Flow network.
type GossipSubAdapterConfig struct {
	options []pubsub.Option
}

var _ p2p.PubSubAdapterConfig = (*GossipSubAdapterConfig)(nil)

func NewGossipSubAdapterConfig(base *p2p.BasePubSubAdapterConfig) *GossipSubAdapterConfig {
	return &GossipSubAdapterConfig{
		options: defaultPubsubOptions(base),
	}
}

func (g *GossipSubAdapterConfig) WithRoutingDiscovery(routing routing.ContentRouting) {
	g.options = append(g.options, pubsub.WithDiscovery(discoveryrouting.NewRoutingDiscovery(routing)))
}

func (g *GossipSubAdapterConfig) WithSubscriptionFilter(filter p2p.SubscriptionFilter) {
	g.options = append(g.options, pubsub.WithSubscriptionFilter(filter))
}

func (g *GossipSubAdapterConfig) WithScoreOption(option p2p.ScoreOption) {
	g.options = append(g.options, option.BuildFlowPubSubScoreOption())
}

func (g *GossipSubAdapterConfig) WithMessageIdFunction(f func([]byte) string) {
	g.options = append(g.options, pubsub.WithMessageIdFn(func(pmsg *pb.Message) string {
		return f(pmsg.Data)
	}))
}

func (g *GossipSubAdapterConfig) Build() []pubsub.Option {
	return g.options
}

func defaultPubsubOptions(base *p2p.BasePubSubAdapterConfig) []pubsub.Option {
	return []pubsub.Option{
		pubsub.WithMessageSigning(true),
		pubsub.WithStrictSignatureVerification(true),
		pubsub.WithMaxMessageSize(base.MaxMessageSize),
	}
}
