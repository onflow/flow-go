package p2pnode

import (
	"context"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"

	"github.com/onflow/flow-go/network/p2p"
)

type GossipSubAdapter struct {
	gossipSub *pubsub.PubSub
	cr        routing.ContentRouting
}

var _ p2p.PubSubAdapter = (*GossipSubAdapter)(nil)

func NewGossipSubAdapter(ctx context.Context, h host.Host, opts ...pubsub.Option) (p2p.PubSubAdapter, error) {
	gossipSub, err := pubsub.NewGossipSub(ctx, h, opts...)
	if err != nil {
		return nil, err
	}
	return &GossipSubAdapter{
		gossipSub: gossipSub,
	}, nil
}

func (g *GossipSubAdapter) RegisterTopicValidator(topic string, val interface{}) error {
	return g.gossipSub.RegisterTopicValidator(topic, val, pubsub.WithValidatorInline(true))
}

func (g *GossipSubAdapter) UnregisterTopicValidator(topic string) error {
	return g.gossipSub.UnregisterTopicValidator(topic)
}

func (g *GossipSubAdapter) Join(topic string) (p2p.Topic, error) {
	t, err := g.gossipSub.Join(topic)
	if err != nil {
		return nil, err
	}
	return NewGossipSubTopic(t), nil
}

func (g *GossipSubAdapter) GetTopics() []string {
	return g.gossipSub.GetTopics()
}

func (g *GossipSubAdapter) ListPeers(topic string) []peer.ID {
	return g.gossipSub.ListPeers(topic)
}

func defaultPubsubOptions() []pubsub.Option {

}