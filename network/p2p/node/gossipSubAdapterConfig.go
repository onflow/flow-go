package p2pnode

import (
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	discoveryrouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"

	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/network/p2p"
)

// GossipSubAdapterConfig is a wrapper around libp2p pubsub options that
// implements the PubSubAdapterConfig interface for the Flow network.
type GossipSubAdapterConfig struct {
	options      []pubsub.Option
	scoreTracer  p2p.PeerScoreTracer
	scoreOption  p2p.ScoreOptionBuilder
	pubsubTracer p2p.PubSubTracer
	inspector    p2p.GossipSubRPCInspector // currently only used to manage the lifecycle.
}

var _ p2p.PubSubAdapterConfig = (*GossipSubAdapterConfig)(nil)

// NewGossipSubAdapterConfig creates a new GossipSubAdapterConfig with the default options.
// Args:
//   - base: the base pubsub adapter config
//
// Returns:
//   - a new GossipSubAdapterConfig
func NewGossipSubAdapterConfig(base *p2p.BasePubSubAdapterConfig) *GossipSubAdapterConfig {
	return &GossipSubAdapterConfig{
		options: defaultPubsubOptions(base),
	}
}

// WithRoutingDiscovery adds a routing discovery option to the config.
// Args:
//   - routing: the routing discovery to use
//
// Returns:
// -None
func (g *GossipSubAdapterConfig) WithRoutingDiscovery(routing routing.ContentRouting) {
	g.options = append(g.options, pubsub.WithDiscovery(discoveryrouting.NewRoutingDiscovery(routing)))
}

// WithSubscriptionFilter adds a subscription filter option to the config.
// Args:
//   - filter: the subscription filter to use
//
// Returns:
// -None
func (g *GossipSubAdapterConfig) WithSubscriptionFilter(filter p2p.SubscriptionFilter) {
	g.options = append(g.options, pubsub.WithSubscriptionFilter(filter))
}

// WithScoreOption adds a score option to the config.
// Args:
// - option: the score option to use
// Returns:
// -None
func (g *GossipSubAdapterConfig) WithScoreOption(option p2p.ScoreOptionBuilder) {
	params, thresholds := option.BuildFlowPubSubScoreOption()
	g.scoreOption = option
	g.options = append(g.options, pubsub.WithPeerScore(params, thresholds))
}

// WithMessageIdFunction adds a message ID function option to the config.
// Args:
// - f: the message ID function to use
// Returns:
// -None
func (g *GossipSubAdapterConfig) WithMessageIdFunction(f func([]byte) string) {
	g.options = append(g.options, pubsub.WithMessageIdFn(func(pmsg *pb.Message) string {
		return f(pmsg.Data)
	}))
}

// WithInspectorSuite adds an inspector suite option to the config.
// Args:
// - suite: the inspector suite to use
// Returns:
// -None
func (g *GossipSubAdapterConfig) WithRpcInspector(inspector p2p.GossipSubMsgValidationRpcInspector) {
	g.options = append(g.options, pubsub.WithAppSpecificRpcInspector(inspector.Inspect))
	g.inspector = inspector
}

// WithTracer adds a tracer option to the config.
// Args:
// - tracer: the tracer to use
// Returns:
// -None
func (g *GossipSubAdapterConfig) WithTracer(tracer p2p.PubSubTracer) {
	g.pubsubTracer = tracer
	g.options = append(g.options, pubsub.WithRawTracer(tracer))
}

// ScoreTracer returns the tracer for the peer score.
// Args:
//   - None
//
// Returns:
// - p2p.PeerScoreTracer: the tracer for the peer score.
func (g *GossipSubAdapterConfig) ScoreTracer() p2p.PeerScoreTracer {
	return g.scoreTracer
}

// PubSubTracer returns the tracer for the pubsub.
// Args:
// - None
// Returns:
// - p2p.PubSubTracer: the tracer for the pubsub.
func (g *GossipSubAdapterConfig) PubSubTracer() p2p.PubSubTracer {
	return g.pubsubTracer
}

func (g *GossipSubAdapterConfig) ScoringComponent() component.Component {
	return g.scoreOption
}

// RpcInspectorComponent returns the component that manages the lifecycle of the inspector suite.
// This is used to start and stop the inspector suite by the PubSubAdapter.
// Args:
//   - None
//
// Returns:
//   - component.Component: the component that manages the lifecycle of the inspector suite.
func (g *GossipSubAdapterConfig) RpcInspectorComponent() component.Component {
	return g.inspector
}

// WithScoreTracer sets the tracer for the peer score.
// Args:
//   - tracer: the tracer for the peer score.
//
// Returns:
//   - None
func (g *GossipSubAdapterConfig) WithScoreTracer(tracer p2p.PeerScoreTracer) {
	g.scoreTracer = tracer
	g.options = append(g.options, pubsub.WithPeerScoreInspect(func(snapshot map[peer.ID]*pubsub.PeerScoreSnapshot) {
		tracer.UpdatePeerScoreSnapshots(convertPeerScoreSnapshots(snapshot))
	}, tracer.UpdateInterval()))
}

// convertPeerScoreSnapshots converts a libp2p pubsub peer score snapshot to a Flow peer score snapshot.
// Args:
//   - snapshot: the libp2p pubsub peer score snapshot.
//
// Returns:
//   - map[peer.ID]*p2p.PeerScoreSnapshot: the Flow peer score snapshot.
func convertPeerScoreSnapshots(snapshot map[peer.ID]*pubsub.PeerScoreSnapshot) map[peer.ID]*p2p.PeerScoreSnapshot {
	newSnapshot := make(map[peer.ID]*p2p.PeerScoreSnapshot)
	for id, snap := range snapshot {
		newSnapshot[id] = &p2p.PeerScoreSnapshot{
			Topics:             convertTopicScoreSnapshot(snap.Topics),
			Score:              snap.Score,
			AppSpecificScore:   snap.AppSpecificScore,
			BehaviourPenalty:   snap.BehaviourPenalty,
			IPColocationFactor: snap.IPColocationFactor,
		}
	}
	return newSnapshot
}

// convertTopicScoreSnapshot converts a libp2p pubsub topic score snapshot to a Flow topic score snapshot.
// Args:
//   - snapshot: the libp2p pubsub topic score snapshot.
//
// Returns:
//   - map[string]*p2p.TopicScoreSnapshot: the Flow topic score snapshot.
func convertTopicScoreSnapshot(snapshot map[string]*pubsub.TopicScoreSnapshot) map[string]*p2p.TopicScoreSnapshot {
	newSnapshot := make(map[string]*p2p.TopicScoreSnapshot)
	for topic, snap := range snapshot {
		newSnapshot[topic] = &p2p.TopicScoreSnapshot{
			TimeInMesh:               snap.TimeInMesh,
			FirstMessageDeliveries:   snap.FirstMessageDeliveries,
			MeshMessageDeliveries:    snap.MeshMessageDeliveries,
			InvalidMessageDeliveries: snap.InvalidMessageDeliveries,
		}
	}

	return newSnapshot
}

// TopicScoreParamFunc returns the topic score param function. This function is used to get the topic score params for a topic.
// The topic score params are used to set the topic parameters in GossipSub at the time of joining the topic.
// Args:
//   - None
//
// Returns:
// - func(topic *pubsub.Topic) *pubsub.TopicScoreParams: the topic score param function if set, nil otherwise.
// - bool: true if the topic score param function is set, false otherwise.
func (g *GossipSubAdapterConfig) TopicScoreParamFunc() (func(topic *pubsub.Topic) *pubsub.TopicScoreParams, bool) {
	if g.scoreOption != nil {
		return func(topic *pubsub.Topic) *pubsub.TopicScoreParams {
			return g.scoreOption.TopicScoreParams(topic)
		}, true
	}

	return nil, false
}

// Build returns the libp2p pubsub options.
// Args:
//   - None
//
// Returns:
//   - []pubsub.Option: the libp2p pubsub options.
//
// Build is idempotent.
func (g *GossipSubAdapterConfig) Build() []pubsub.Option {
	return g.options
}

// defaultPubsubOptions returns the default libp2p pubsub options. These options are used by the Flow network to create a libp2p pubsub.
func defaultPubsubOptions(base *p2p.BasePubSubAdapterConfig) []pubsub.Option {
	return []pubsub.Option{
		pubsub.WithMessageSigning(true),
		pubsub.WithStrictSignatureVerification(true),
		pubsub.WithMaxMessageSize(base.MaxMessageSize),
	}
}
