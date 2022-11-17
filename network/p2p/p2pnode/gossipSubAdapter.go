package p2pnode

import (
	"context"
	"fmt"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/network/p2p"
)

type GossipSubAdapter struct {
	gossipSub *pubsub.PubSub
}

var _ p2p.PubSubAdapter = (*GossipSubAdapter)(nil)

func NewGossipSubAdapter(ctx context.Context, h host.Host, cfg p2p.PubSubAdapterConfig) (p2p.PubSubAdapter, error) {
	gossipSubConfig, ok := cfg.(*GossipSubAdapterConfig)
	if !ok {
		return nil, fmt.Errorf("invalid gossipsub config type: %T", cfg)
	}

	gossipSub, err := pubsub.NewGossipSub(ctx, h, gossipSubConfig.Build()...)
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
