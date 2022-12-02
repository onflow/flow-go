package corruptlibp2p

import (
	"context"
	"fmt"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"
	corrupt "github.com/yhassanzadeh13/go-libp2p-pubsub"

	"github.com/onflow/flow-go/insecure/internal"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/utils/logging"
)

// CorruptGossipSubAdapter is a wrapper around the forked pubsub topic from
// github.com/yhassanzadeh13/go-libp2p-pubsub that implements the p2p.PubSubAdapter.
// This is needed because in order to use the forked pubsub module, we need to
// use the entire dependency tree of the forked module which is resolved to
// github.com/yhassanzadeh13/go-libp2p-pubsub. This means that we cannot use
// the original libp2p pubsub module in the same package.
// Note: we use the forked pubsub module for sake of BFT testing and attack vector
// implementation, it is designed to be completely isolated in the "insecure" package, and
// totally separated from the rest of the codebase.
type CorruptGossipSubAdapter struct {
	gossipSub *corrupt.PubSub
	router    *internal.CorruptGossipSubRouter
	logger    zerolog.Logger
}

var _ p2p.PubSubAdapter = (*CorruptGossipSubAdapter)(nil)

func (c *CorruptGossipSubAdapter) RegisterTopicValidator(topic string, topicValidator p2p.TopicValidatorFunc) error {
	// instantiates a corrupt.ValidatorEx that wraps the topicValidatorFunc
	var corruptValidator corrupt.ValidatorEx = func(ctx context.Context, from peer.ID, message *corrupt.Message) corrupt.ValidationResult {
		pubsubMsg := &pubsub.Message{
			Message:       message.Message, // converting corrupt.Message to pubsub.Message
			ID:            message.ID,
			ReceivedFrom:  message.ReceivedFrom,
			ValidatorData: message.ValidatorData,
			Local:         message.Local,
		}
		result := topicValidator(ctx, from, pubsubMsg)

		// overriding the corrupt.ValidationResult with the result from pubsub.TopicValidatorFunc
		message.ValidatorData = pubsubMsg.ValidatorData

		switch result {
		case p2p.ValidationAccept:
			return corrupt.ValidationAccept
		case p2p.ValidationIgnore:
			return corrupt.ValidationIgnore
		case p2p.ValidationReject:
			return corrupt.ValidationReject
		default:
			// should never happen, indicates a bug in the topic validator
			c.logger.Fatal().
				Bool(logging.KeySuspicious, true).
				Str("topic", topic).
				Str("origin_peer", from.String()).
				Str("result", fmt.Sprintf("%v", result)).
				Str("message_type", fmt.Sprintf("%T", message.Data)).
				Msgf("invalid validation result, should be a bug in the topic validator")
		}
		// should never happen, indicates a bug in the topic validator, but we need to return something
		c.logger.Warn().
			Bool(logging.KeySuspicious, true).
			Str("topic", topic).
			Str("origin_peer", from.String()).
			Str("result", fmt.Sprintf("%v", result)).
			Str("message_type", fmt.Sprintf("%T", message.Data)).
			Msg("invalid validation result, returning reject")
		return corrupt.ValidationReject
	}
	err := c.gossipSub.RegisterTopicValidator(topic, corruptValidator, corrupt.WithValidatorInline(true))
	if err != nil {
		return fmt.Errorf("could not register topic validator on corrupt gossipsub: %w", err)
	}
	return nil
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

func NewCorruptGossipSubAdapter(ctx context.Context, logger zerolog.Logger, h host.Host, cfg p2p.PubSubAdapterConfig) (p2p.PubSubAdapter, *internal.CorruptGossipSubRouter, error) {
	gossipSubConfig, ok := cfg.(*CorruptPubSubAdapterConfig)
	if !ok {
		return nil, nil, fmt.Errorf("invalid gossipsub config type: %T", cfg)
	}

	// initializes a default gossipsub router and wraps it with the corrupt router.
	router, err := corrupt.DefaultGossipSubRouter(h)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create gossipsub router: %w", err)
	}
	corruptRouter := internal.NewCorruptGossipSubRouter(router)

	// injects the corrupt router into the gossipsub constructor
	gossipSub, err := corrupt.NewGossipSubWithRouter(ctx, h, corruptRouter, gossipSubConfig.Build()...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create corrupt gossipsub: %w", err)
	}

	adapter := &CorruptGossipSubAdapter{
		gossipSub: gossipSub,
		router:    corruptRouter,
		logger:    logger,
	}

	return adapter, corruptRouter, nil
}
