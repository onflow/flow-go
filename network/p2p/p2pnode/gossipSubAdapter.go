package p2pnode

import (
	"context"
	"fmt"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/utils/logging"
)

// GossipSubAdapter is a wrapper around the libp2p GossipSub implementation
// that implements the PubSubAdapter interface for the Flow network.
type GossipSubAdapter struct {
	component.Component
	gossipSub           *pubsub.PubSub
	topicScoreParamFunc func(topic *pubsub.Topic) *pubsub.TopicScoreParams
	logger              zerolog.Logger
	peerScoreExposer    p2p.PeerScoreExposer
}

var _ p2p.PubSubAdapter = (*GossipSubAdapter)(nil)

func NewGossipSubAdapter(ctx context.Context, logger zerolog.Logger, h host.Host, cfg p2p.PubSubAdapterConfig) (p2p.PubSubAdapter, error) {
	gossipSubConfig, ok := cfg.(*GossipSubAdapterConfig)
	if !ok {
		return nil, fmt.Errorf("invalid gossipsub config type: %T", cfg)
	}

	gossipSub, err := pubsub.NewGossipSub(ctx, h, gossipSubConfig.Build()...)
	if err != nil {
		return nil, err
	}

	builder := component.NewComponentManagerBuilder()

	a := &GossipSubAdapter{
		gossipSub: gossipSub,
		logger:    logger.With().Str("component", "gossipsub-adapter").Logger(),
	}

	topicScoreParamFunc, ok := gossipSubConfig.TopicScoreParamFunc()
	if ok {
		a.topicScoreParamFunc = topicScoreParamFunc
	} else {
		a.logger.Warn().Msg("no topic score param func provided")
	}

	if scoreTracer := gossipSubConfig.ScoreTracer(); scoreTracer != nil {
		builder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()
			a.logger.Debug().Str("component", "gossipsub_score_tracer").Msg("starting score tracer")
			scoreTracer.Start(ctx)
			a.logger.Debug().Str("component", "gossipsub_score_tracer").Msg("score tracer started")

			<-scoreTracer.Done()
			a.logger.Debug().Str("component", "gossipsub_score_tracer").Msg("score tracer stopped")
		})
		a.peerScoreExposer = scoreTracer
	}

	if tracer := gossipSubConfig.PubSubTracer(); tracer != nil {
		builder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()
			a.logger.Debug().Str("component", "gossipsub_tracer").Msg("starting tracer")
			tracer.Start(ctx)
			a.logger.Debug().Str("component", "gossipsub_tracer").Msg("tracer started")

			<-tracer.Done()
			a.logger.Debug().Str("component", "gossipsub_tracer").Msg("tracer stopped")
		})
	}

	if inspectorSuite := gossipSubConfig.InspectorSuiteComponent(); inspectorSuite != nil {
		builder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			a.logger.Debug().Str("component", "gossipsub_inspector_suite").Msg("starting inspector suite")
			inspectorSuite.Start(ctx)
			a.logger.Debug().Str("component", "gossipsub_inspector_suite").Msg("inspector suite started")

			select {
			case <-ctx.Done():
				a.logger.Debug().Str("component", "gossipsub_inspector_suite").Msg("inspector suite context done")
			case <-inspectorSuite.Ready():
				ready()
				a.logger.Debug().Str("component", "gossipsub_inspector_suite").Msg("inspector suite ready")
			}

			<-inspectorSuite.Done()
			a.logger.Debug().Str("component", "gossipsub_inspector_suite").Msg("inspector suite stopped")
		})
	}

	a.Component = builder.Build()

	return a, nil
}

func (g *GossipSubAdapter) RegisterTopicValidator(topic string, topicValidator p2p.TopicValidatorFunc) error {
	// wrap the topic validator function into a libp2p topic validator function.
	var v pubsub.ValidatorEx = func(ctx context.Context, from peer.ID, message *pubsub.Message) pubsub.ValidationResult {
		switch result := topicValidator(ctx, from, message); result {
		case p2p.ValidationAccept:
			return pubsub.ValidationAccept
		case p2p.ValidationIgnore:
			return pubsub.ValidationIgnore
		case p2p.ValidationReject:
			return pubsub.ValidationReject
		default:
			// should never happen, indicates a bug in the topic validator
			g.logger.Fatal().Msgf("invalid validation result: %v", result)
		}
		// should never happen, indicates a bug in the topic validator, but we need to return something
		g.logger.Warn().
			Bool(logging.KeySuspicious, true).
			Msg("invalid validation result, returning reject")
		return pubsub.ValidationReject
	}

	return g.gossipSub.RegisterTopicValidator(topic, v, pubsub.WithValidatorInline(true))
}

func (g *GossipSubAdapter) UnregisterTopicValidator(topic string) error {
	return g.gossipSub.UnregisterTopicValidator(topic)
}

func (g *GossipSubAdapter) Join(topic string) (p2p.Topic, error) {
	t, err := g.gossipSub.Join(topic)
	if err != nil {
		return nil, fmt.Errorf("could not join topic %s: %w", topic, err)
	}

	if g.topicScoreParamFunc != nil {
		topicParams := g.topicScoreParamFunc(t)
		err = t.SetScoreParams(topicParams)
		if err != nil {
			return nil, fmt.Errorf("could not set score params for topic %s: %w", topic, err)
		}
		g.logger.Info().Str("topic", topic).
			Bool("atomic_validation", topicParams.SkipAtomicValidation).
			Float64("topic_weight", topicParams.TopicWeight).
			Float64("time_in_mesh_weight", topicParams.TimeInMeshWeight).
			Dur("time_in_mesh_quantum", topicParams.TimeInMeshQuantum).
			Float64("time_in_mesh_cap", topicParams.TimeInMeshCap).
			Float64("first_message_deliveries_weight", topicParams.FirstMessageDeliveriesWeight).
			Float64("first_message_deliveries_decay", topicParams.FirstMessageDeliveriesDecay).
			Float64("first_message_deliveries_cap", topicParams.FirstMessageDeliveriesCap).
			Float64("mesh_message_deliveries_weight", topicParams.MeshMessageDeliveriesWeight).
			Float64("mesh_message_deliveries_decay", topicParams.MeshMessageDeliveriesDecay).
			Float64("mesh_message_deliveries_cap", topicParams.MeshMessageDeliveriesCap).
			Float64("mesh_message_deliveries_threshold", topicParams.MeshMessageDeliveriesThreshold).
			Dur("mesh_message_deliveries_window", topicParams.MeshMessageDeliveriesWindow).
			Dur("mesh_message_deliveries_activation", topicParams.MeshMessageDeliveriesActivation).
			Float64("mesh_failure_penalty_weight", topicParams.MeshFailurePenaltyWeight).
			Float64("mesh_failure_penalty_decay", topicParams.MeshFailurePenaltyDecay).
			Float64("invalid_message_deliveries_weight", topicParams.InvalidMessageDeliveriesWeight).
			Float64("invalid_message_deliveries_decay", topicParams.InvalidMessageDeliveriesDecay).
			Msg("joined topic with score params set")
	} else {
		g.logger.Warn().
			Str("topic", topic).
			Msg("joining topic without score params, this is not recommended from a security perspective")
	}
	return NewGossipSubTopic(t), nil
}

func (g *GossipSubAdapter) GetTopics() []string {
	return g.gossipSub.GetTopics()
}

func (g *GossipSubAdapter) ListPeers(topic string) []peer.ID {
	return g.gossipSub.ListPeers(topic)
}

// PeerScoreExposer returns the peer score exposer for the gossipsub adapter. The exposer is a read-only interface
// for querying peer scores.
// The exposer is only available if the gossipsub adapter was configured with a score tracer.
// If the gossipsub adapter was not configured with a score tracer, the exposer will be nil.
// Args:
//
//	None.
//
// Returns:
//
//	The peer score exposer for the gossipsub adapter.
func (g *GossipSubAdapter) PeerScoreExposer() p2p.PeerScoreExposer {
	return g.peerScoreExposer
}
