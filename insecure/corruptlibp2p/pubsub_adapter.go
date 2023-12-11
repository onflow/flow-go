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
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/p2plogging"
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
	component.Component
	gossipSub             *corrupt.PubSub
	router                *corrupt.GossipSubRouter
	logger                zerolog.Logger
	clusterChangeConsumer p2p.CollectionClusterChangesConsumer
	peerScoreExposer      p2p.PeerScoreExposer
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
				Str("origin_peer", p2plogging.PeerId(from)).
				Str("result", fmt.Sprintf("%v", result)).
				Str("message_type", fmt.Sprintf("%T", message.Data)).
				Msgf("invalid validation result, should be a bug in the topic validator")
		}
		// should never happen, indicates a bug in the topic validator, but we need to return something
		c.logger.Warn().
			Bool(logging.KeySuspicious, true).
			Str("topic", topic).
			Str("origin_peer", p2plogging.PeerId(from)).
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

func (c *CorruptGossipSubAdapter) GetLocalMeshPeers(topic channels.Topic) []peer.ID {
	// this method is a no-op in the corrupt gossipsub; as the corrupt gossipsub is solely used for testing, it does not come with a mesh tracer.
	return []peer.ID{}
}

func (c *CorruptGossipSubAdapter) ActiveClustersChanged(lst flow.ChainIDList) {
	c.clusterChangeConsumer.ActiveClustersChanged(lst)
}

// PeerScoreExposer returns the peer score exposer for the gossipsub adapter. The exposer is a read-only interface
// for querying peer scores and returns the local scoring table of the underlying gossipsub node.
// The exposer is only available if the gossipsub adapter was configured with a score tracer.
// If the gossipsub adapter was not configured with a score tracer, the exposer will be nil.
// Args:
//
//	None.
//
// Returns:
//
//	The peer score exposer for the gossipsub adapter.
func (c *CorruptGossipSubAdapter) PeerScoreExposer() p2p.PeerScoreExposer {
	return c.peerScoreExposer
}

func NewCorruptGossipSubAdapter(ctx context.Context,
	logger zerolog.Logger,
	h host.Host,
	cfg p2p.PubSubAdapterConfig,
	clusterChangeConsumer p2p.CollectionClusterChangesConsumer) (p2p.PubSubAdapter, *corrupt.GossipSubRouter, error) {
	gossipSubConfig, ok := cfg.(*CorruptPubSubAdapterConfig)
	if !ok {
		return nil, nil, fmt.Errorf("invalid gossipsub config type: %T", cfg)
	}

	// initializes a default gossipsub router and wraps it with the corrupt router.
	router := corrupt.DefaultGossipSubRouter(h)

	// injects the corrupt router into the gossipsub constructor
	gossipSub, err := corrupt.NewGossipSubWithRouter(ctx, h, router, gossipSubConfig.Build()...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create corrupt gossipsub: %w", err)
	}

	builder := component.NewComponentManagerBuilder()
	adapter := &CorruptGossipSubAdapter{
		gossipSub:             gossipSub,
		router:                router,
		logger:                logger,
		clusterChangeConsumer: clusterChangeConsumer,
	}

	if scoreTracer := gossipSubConfig.ScoreTracer(); scoreTracer != nil {
		builder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()
			logger.Debug().Str("component", "corrupt-gossipsub_score_tracer").Msg("starting score tracer")
			scoreTracer.Start(ctx)
			logger.Debug().Str("component", "corrupt-gossipsub_score_tracer").Msg("score tracer started")

			<-scoreTracer.Done()
			logger.Debug().Str("component", "corrupt-gossipsub_score_tracer").Msg("score tracer stopped")
		})
		adapter.peerScoreExposer = scoreTracer
	}
	builder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
		ready()
		// it is likely that this adapter is configured without a score tracer, so we need to
		// wait for the context to be done in order to prevent immature shutdown.
		<-ctx.Done()
	})

	adapter.Component = builder.Build()

	return adapter, router, nil
}
