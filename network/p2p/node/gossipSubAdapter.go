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
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/utils"
	"github.com/onflow/flow-go/utils/logging"
)

// GossipSubAdapter is a wrapper around the libp2p GossipSub implementation
// that implements the PubSubAdapter interface for the Flow network.
type GossipSubAdapter struct {
	component.Component
	gossipSub *pubsub.PubSub
	// topicScoreParamFunc is a function that returns the topic score params for a given topic.
	// If no function is provided the node will join the topic with no scoring params. As the
	// node will not be able to score other peers in the topic, it may be vulnerable to routing
	// attacks on the topic that may also affect the overall function of the node.
	// It is not recommended to use this adapter without a topicScoreParamFunc. Also in mature
	// implementations of the Flow network, the topicScoreParamFunc must be a required parameter.
	topicScoreParamFunc func(topic *pubsub.Topic) *pubsub.TopicScoreParams
	logger              zerolog.Logger
	peerScoreExposer    p2p.PeerScoreExposer
	localMeshTracer     p2p.PubSubTracer
	// clusterChangeConsumer is a callback that is invoked when the set of active clusters of collection nodes changes.
	// This callback is implemented by the rpc inspector suite of the GossipSubAdapter, and consumes the cluster changes
	// to update the rpc inspector state of the recent topics (i.e., channels).
	clusterChangeConsumer p2p.CollectionClusterChangesConsumer
}

var _ p2p.PubSubAdapter = (*GossipSubAdapter)(nil)

func NewGossipSubAdapter(ctx context.Context,
	logger zerolog.Logger,
	h host.Host,
	cfg p2p.PubSubAdapterConfig,
	clusterChangeConsumer p2p.CollectionClusterChangesConsumer) (p2p.PubSubAdapter, error) {
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
		logger:                logger.With().Str("component", "gossipsub-adapter").Logger(),
		clusterChangeConsumer: clusterChangeConsumer,
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
			a.logger.Info().Msg("starting score tracer")
			scoreTracer.Start(ctx)
			select {
			case <-ctx.Done():
				a.logger.Warn().Msg("aborting score tracer startup due to context done")
			case <-scoreTracer.Ready():
				a.logger.Info().Msg("score tracer is ready")
			}
			ready()

			<-ctx.Done()
			a.logger.Info().Msg("stopping score tracer")
			<-scoreTracer.Done()
			a.logger.Info().Msg("score tracer stopped")
		})
		a.peerScoreExposer = scoreTracer
	}

	if tracer := gossipSubConfig.PubSubTracer(); tracer != nil {
		builder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			a.logger.Info().Msg("starting pubsub tracer")
			tracer.Start(ctx)
			select {
			case <-ctx.Done():
				a.logger.Warn().Msg("aborting pubsub tracer startup due to context done")
			case <-tracer.Ready():
				a.logger.Info().Msg("pubsub tracer is ready")
			}
			ready()

			<-ctx.Done()
			a.logger.Info().Msg("stopping pubsub tracer")
			<-tracer.Done()
			a.logger.Info().Msg("pubsub tracer stopped")
		})
		a.localMeshTracer = tracer
	}

	if inspectorSuite := gossipSubConfig.InspectorSuiteComponent(); inspectorSuite != nil {
		builder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			a.logger.Info().Msg("starting inspector suite")
			inspectorSuite.Start(ctx)
			select {
			case <-ctx.Done():
				a.logger.Warn().Msg("aborting inspector suite startup due to context done")
			case <-inspectorSuite.Ready():
				a.logger.Info().Msg("inspector suite is ready")
			}
			ready()

			<-ctx.Done()
			a.logger.Info().Msg("stopping inspector suite")
			<-inspectorSuite.Done()
			a.logger.Info().Msg("inspector suite stopped")
		})
	}

	if scoringComponent := gossipSubConfig.ScoringComponent(); scoringComponent != nil {
		builder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			a.logger.Info().Msg("starting gossipsub scoring component")
			scoringComponent.Start(ctx)
			select {
			case <-ctx.Done():
				a.logger.Warn().Msg("aborting gossipsub scoring component startup due to context done")
			case <-scoringComponent.Ready():
				a.logger.Info().Msg("gossipsub scoring component is ready")
			}
			ready()

			<-ctx.Done()
			a.logger.Info().Msg("stopping gossipsub scoring component")
			<-scoringComponent.Done()
			a.logger.Info().Msg("gossipsub scoring component stopped")
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
		topicParamsLogger := utils.TopicScoreParamsLogger(g.logger, topic, topicParams)
		topicParamsLogger.Info().Msg("joined topic with score params set")
	} else {
		g.logger.Warn().
			Bool(logging.KeyNetworkingSecurity, true).
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

// GetLocalMeshPeers returns the list of peers in the local mesh for the given topic.
// Args:
// - topic: the topic.
// Returns:
// - []peer.ID: the list of peers in the local mesh for the given topic.
func (g *GossipSubAdapter) GetLocalMeshPeers(topic channels.Topic) []peer.ID {
	return g.localMeshTracer.GetLocalMeshPeers(topic)
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
func (g *GossipSubAdapter) PeerScoreExposer() p2p.PeerScoreExposer {
	return g.peerScoreExposer
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
