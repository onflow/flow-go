package corruptlibp2p

import (
	"context"
	"fmt"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"
	corrupt "github.com/yhassanzadeh13/go-libp2p-pubsub"

	"github.com/onflow/flow-go/insecure/corruptlibp2p/internal"
	"github.com/onflow/flow-go/network/p2p"
)

type CorruptGossipSubAdapter struct {
	gossipSub *corrupt.PubSub
	router    *internal.CorruptGossipSubRouter
	logger    zerolog.Logger
}

var _ p2p.PubSubAdapter = (*CorruptGossipSubAdapter)(nil)

func (c *CorruptGossipSubAdapter) RegisterTopicValidator(topic string, topicValidator p2p.TopicValidatorFunc) error {
	var v corrupt.ValidatorEx = func(ctx context.Context, from peer.ID, message *corrupt.Message) corrupt.ValidationResult {
		switch result := topicValidator(ctx, from, &pubsub.Message{
			Message:       message.Message, // converting corrupt.Message to pubsub.Message
			ID:            message.ID,
			ReceivedFrom:  message.ReceivedFrom,
			ValidatorData: message.ValidatorData,
			Local:         message.Local,
		}); result {
		case p2p.ValidationAccept:
			return corrupt.ValidationAccept
		case p2p.ValidationIgnore:
			return corrupt.ValidationIgnore
		case p2p.ValidationReject:
			return corrupt.ValidationReject
		default:
			// should never happen, indicates a bug in the topic validator
			c.logger.Fatal().Msgf("invalid validation result: %v", result)
		}
		// should never happen, indicates a bug in the topic validator, but we need to return something
		c.logger.Warn().Msg("invalid validation result, returning reject")
		return corrupt.ValidationReject
	}
	return c.gossipSub.RegisterTopicValidator(topic, v, corrupt.WithValidatorInline(true))
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
	return c.gossipSub.ListPeers(topic)
}

func (c *CorruptGossipSubAdapter) GetRouter() *internal.CorruptGossipSubRouter {
	return c.router
}

func NewCorruptGossipSubAdapter(ctx context.Context, logger zerolog.Logger, h host.Host, cfg p2p.PubSubAdapterConfig) (p2p.PubSubAdapter, error) {
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
		logger:    logger,
	}, nil
}
