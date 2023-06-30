package p2pnode

import (
	"context"
	"fmt"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/utils/logging"
)

// GossipSubAdapter is a wrapper around the libp2p GossipSub implementation
// that implements the PubSubAdapter interface for the Flow network.
type GossipSubAdapter struct {
	component.Component
	gossipSub *pubsub.PubSub
	logger    zerolog.Logger
	// clusterChangeConsumer is a callback that is invoked when the set of active clusters of collection nodes changes.
	// This callback is implemented by the rpc inspector suite of the GossipSubAdapter, and consumes the cluster changes
	// to update the rpc inspector state of the recent topics (i.e., channels).
	clusterChangeConsumer p2p.CollectionClusterChangesConsumer
}

var _ p2p.PubSubAdapter = (*GossipSubAdapter)(nil)

func NewGossipSubAdapter(ctx context.Context, logger zerolog.Logger, h host.Host, cfg p2p.PubSubAdapterConfig, clusterChangeConsumer p2p.CollectionClusterChangesConsumer) (p2p.PubSubAdapter, error) {
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
		gossipSub:             gossipSub,
		logger:                logger,
		clusterChangeConsumer: clusterChangeConsumer,
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
	return NewGossipSubTopic(t), nil
}

func (g *GossipSubAdapter) GetTopics() []string {
	return g.gossipSub.GetTopics()
}

func (g *GossipSubAdapter) ListPeers(topic string) []peer.ID {
	return g.gossipSub.ListPeers(topic)
}

// ActiveClustersChanged is called when the active clusters of collection nodes changes.
// GossipSubAdapter implements this method to forward the call to the clusterChangeConsumer (rpc inspector),
// which will then update the cluster state of the rpc inspector.
// Args:
// - lst: the list of active clusters
// Returns:
// - void
func (g *GossipSubAdapter) ActiveClustersChanged(lst flow.ChainIDList) {
	g.clusterChangeConsumer.ActiveClustersChanged(lst)
}
