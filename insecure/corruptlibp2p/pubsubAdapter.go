package corruptlibp2p

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	corrupt "github.com/yhassanzadeh13/go-libp2p-pubsub"

	"github.com/onflow/flow-go/insecure/corruptlibp2p/internal"
	"github.com/onflow/flow-go/network/p2p"
)

type CorruptGossipSubAdapter struct {
	gossipSub *corrupt.PubSub
	router    *internal.CorruptGossipSubRouter
}

var _ p2p.PubSubAdapter = (*CorruptGossipSubAdapter)(nil)

func (c *CorruptGossipSubAdapter) RegisterTopicValidator(topic string, val interface{}) error {
	return c.gossipSub.RegisterTopicValidator(topic, val, corrupt.WithValidatorInline(true))
}

func (c *CorruptGossipSubAdapter) UnregisterTopicValidator(topic string) error {
	return c.gossipSub.UnregisterTopicValidator(topic)
}

func (c *CorruptGossipSubAdapter) Join(topic string) (p2p.Topic, error) {
	t, err := c.gossipSub.Join(topic)
	if err != nil {
		return nil, err
	}
	return internal.NewCorruptTopic(t), nil
}

func (c *CorruptGossipSubAdapter) GetTopics() []string {
	return c.gossipSub.GetTopics()
}

func (c *CorruptGossipSubAdapter) ListPeers(topic string) []peer.ID {
	return c.ListPeers(topic)
}

func (c *CorruptGossipSubAdapter) GetRouter() *internal.CorruptGossipSubRouter {
	return c.router
}

func NewCorruptGossipSubAdapter(ctx context.Context, h host.Host, cfg p2p.PubSubAdapterConfig) (p2p.PubSubAdapter, error) {
	gossipSubConfig, ok := cfg.(*CorruptPubSubAdapterConfig)
	if !ok {
		return nil, fmt.Errorf("invalid gossipsub config type: %T", cfg)
	}

	router, err := corrupt.DefaultGossipSubRouter(h)
	if err != nil {
		return nil, fmt.Errorf("failed to create gossipsub router: %w", err)
	}
	corruptRouter := internal.NewCorruptGossipSubRouter(router)
	gossipSub, err := corrupt.NewGossipSubWithRouter(ctx, h, corruptRouter, gossipSubConfig.Build()...)
	if err != nil {
		return nil, err
	}

	return &CorruptGossipSubAdapter{
		gossipSub: gossipSub,
		router:    corruptRouter,
	}, nil
}
