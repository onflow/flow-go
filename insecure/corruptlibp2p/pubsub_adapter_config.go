package corruptlibp2p

import (
	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	discoveryRouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	corrupt "github.com/yhassanzadeh13/go-libp2p-pubsub"

	"github.com/onflow/flow-go/network/p2p"
)

// CorruptPubSubAdapterConfig is a wrapper around the forked pubsub topic from
// github.com/yhassanzadeh13/go-libp2p-pubsub that implements the p2p.PubSubAdapterConfig.
// This is needed because in order to use the forked pubsub module, we need to
// use the entire dependency tree of the forked module which is resolved to
// github.com/yhassanzadeh13/go-libp2p-pubsub. This means that we cannot use
// the original libp2p pubsub module in the same package.
// Note: we use the forked pubsub module for sake of BFT testing and attack vector
// implementation, it is designed to be completely isolated in the "insecure" package, and
// totally separated from the rest of the codebase.
type CorruptPubSubAdapterConfig struct {
	options                         []corrupt.Option
	inspector                       func(peer.ID, *corrupt.RPC) error
	withMessageSigning              bool
	withStrictSignatureVerification bool
	scoreTracer                     p2p.PeerScoreTracer
}

type CorruptPubSubAdapterConfigOption func(config *CorruptPubSubAdapterConfig)

// WithMessageSigning overrides the libp2p node message signing option. This option can be used to enable or disable message signing.
func WithMessageSigning(withMessageSigning bool) CorruptPubSubAdapterConfigOption {
	return func(config *CorruptPubSubAdapterConfig) {
		config.withMessageSigning = withMessageSigning
	}
}

// WithStrictSignatureVerification overrides the libp2p node message signature verification option. This option can be used to enable or disable message signature verification.
func WithStrictSignatureVerification(withStrictSignatureVerification bool) CorruptPubSubAdapterConfigOption {
	return func(config *CorruptPubSubAdapterConfig) {
		config.withStrictSignatureVerification = withStrictSignatureVerification
	}
}

var _ p2p.PubSubAdapterConfig = (*CorruptPubSubAdapterConfig)(nil)

func WithInspector(inspector func(peer.ID, *corrupt.RPC) error) func(config *CorruptPubSubAdapterConfig) {
	return func(config *CorruptPubSubAdapterConfig) {
		config.inspector = inspector
		config.options = append(config.options, corrupt.WithAppSpecificRpcInspector(func(id peer.ID, rpc *corrupt.RPC) error {
			return config.inspector(id, rpc)
		}))
	}
}

func NewCorruptPubSubAdapterConfig(base *p2p.BasePubSubAdapterConfig, opts ...CorruptPubSubAdapterConfigOption) *CorruptPubSubAdapterConfig {
	config := &CorruptPubSubAdapterConfig{
		withMessageSigning:              true,
		withStrictSignatureVerification: true,
		options:                         make([]corrupt.Option, 0),
	}

	for _, opt := range opts {
		opt(config)
	}

	// Note: we append the default options at the end to make sure that we are not overriding the options provided by the caller.
	config.options = append(config.options, defaultCorruptPubsubOptions(base, config.withMessageSigning, config.withStrictSignatureVerification)...)

	return config
}

func (c *CorruptPubSubAdapterConfig) WithRoutingDiscovery(routing routing.ContentRouting) {
	c.options = append(c.options, corrupt.WithDiscovery(discoveryRouting.NewRoutingDiscovery(routing)))
}

func (c *CorruptPubSubAdapterConfig) WithSubscriptionFilter(filter p2p.SubscriptionFilter) {
	c.options = append(c.options, corrupt.WithSubscriptionFilter(filter))
}

func (c *CorruptPubSubAdapterConfig) WithScoreOption(option p2p.ScoreOptionBuilder) {
	params, thresholds := option.BuildFlowPubSubScoreOption()
	// convert flow pubsub score option to corrupt pubsub score option
	corruptParams := &corrupt.PeerScoreParams{
		SkipAtomicValidation:        params.SkipAtomicValidation,
		TopicScoreCap:               params.TopicScoreCap,
		AppSpecificScore:            params.AppSpecificScore,
		AppSpecificWeight:           params.AppSpecificWeight,
		IPColocationFactorWeight:    params.IPColocationFactorWeight,
		IPColocationFactorThreshold: params.IPColocationFactorThreshold,
		IPColocationFactorWhitelist: params.IPColocationFactorWhitelist,
		BehaviourPenaltyWeight:      params.BehaviourPenaltyWeight,
		BehaviourPenaltyThreshold:   params.BehaviourPenaltyThreshold,
		BehaviourPenaltyDecay:       params.BehaviourPenaltyDecay,
		DecayInterval:               params.DecayInterval,
		DecayToZero:                 params.DecayToZero,
		RetainScore:                 params.RetainScore,
		SeenMsgTTL:                  params.SeenMsgTTL,
	}
	corruptThresholds := &corrupt.PeerScoreThresholds{
		SkipAtomicValidation:        thresholds.SkipAtomicValidation,
		GossipThreshold:             thresholds.GossipThreshold,
		PublishThreshold:            thresholds.PublishThreshold,
		GraylistThreshold:           thresholds.GraylistThreshold,
		AcceptPXThreshold:           thresholds.AcceptPXThreshold,
		OpportunisticGraftThreshold: thresholds.OpportunisticGraftThreshold,
	}
	for topic, topicParams := range params.Topics {
		corruptParams.Topics[topic] = &corrupt.TopicScoreParams{
			SkipAtomicValidation:            topicParams.SkipAtomicValidation,
			TopicWeight:                     topicParams.TopicWeight,
			TimeInMeshWeight:                topicParams.TimeInMeshWeight,
			TimeInMeshQuantum:               topicParams.TimeInMeshQuantum,
			TimeInMeshCap:                   topicParams.TimeInMeshCap,
			FirstMessageDeliveriesWeight:    topicParams.FirstMessageDeliveriesWeight,
			FirstMessageDeliveriesDecay:     topicParams.FirstMessageDeliveriesDecay,
			FirstMessageDeliveriesCap:       topicParams.FirstMessageDeliveriesCap,
			MeshMessageDeliveriesWeight:     topicParams.MeshMessageDeliveriesWeight,
			MeshMessageDeliveriesDecay:      topicParams.MeshMessageDeliveriesDecay,
			MeshMessageDeliveriesCap:        topicParams.MeshMessageDeliveriesCap,
			MeshMessageDeliveriesThreshold:  topicParams.MeshMessageDeliveriesThreshold,
			MeshMessageDeliveriesWindow:     topicParams.MeshMessageDeliveriesWindow,
			MeshMessageDeliveriesActivation: topicParams.MeshMessageDeliveriesActivation,
			MeshFailurePenaltyWeight:        topicParams.MeshFailurePenaltyWeight,
			MeshFailurePenaltyDecay:         topicParams.MeshFailurePenaltyDecay,
			InvalidMessageDeliveriesWeight:  topicParams.InvalidMessageDeliveriesWeight,
			InvalidMessageDeliveriesDecay:   topicParams.InvalidMessageDeliveriesDecay,
		}
	}
	c.options = append(c.options, corrupt.WithPeerScore(corruptParams, corruptThresholds))
}

func (c *CorruptPubSubAdapterConfig) WithTracer(_ p2p.PubSubTracer) {
	// CorruptPubSub does not support tracer. This is a no-op. We can add this if needed,
	// but feature-wise it is not needed for BFT testing and attack vector implementation.
}

func (c *CorruptPubSubAdapterConfig) WithMessageIdFunction(f func([]byte) string) {
	c.options = append(c.options, corrupt.WithMessageIdFn(func(pmsg *pb.Message) string {
		return f(pmsg.Data)
	}))
}

func (c *CorruptPubSubAdapterConfig) WithScoreTracer(tracer p2p.PeerScoreTracer) {
	c.scoreTracer = tracer
	c.options = append(c.options, corrupt.WithPeerScoreInspect(func(snapshot map[peer.ID]*corrupt.PeerScoreSnapshot) {
		tracer.UpdatePeerScoreSnapshots(convertPeerScoreSnapshots(snapshot))
	}, tracer.UpdateInterval()))
}

func (c *CorruptPubSubAdapterConfig) ScoreTracer() p2p.PeerScoreTracer {
	return c.scoreTracer
}

func (c *CorruptPubSubAdapterConfig) WithRpcInspector(_ p2p.GossipSubInspectorSuite) {
	// CorruptPubSub does not support inspector suite. This is a no-op.
}

func (c *CorruptPubSubAdapterConfig) Build() []corrupt.Option {
	return c.options
}

func defaultCorruptPubsubOptions(base *p2p.BasePubSubAdapterConfig, withMessageSigning, withStrictSignatureVerification bool) []corrupt.Option {
	return []corrupt.Option{
		corrupt.WithMessageSigning(withMessageSigning),
		corrupt.WithStrictSignatureVerification(withStrictSignatureVerification),
		corrupt.WithMaxMessageSize(base.MaxMessageSize),
	}
}

// convertPeerScoreSnapshots converts a libp2p pubsub peer score snapshot to a Flow peer score snapshot.
// Args:
//   - snapshot: the libp2p pubsub peer score snapshot.
//
// Returns:
//   - map[peer.ID]*p2p.PeerScoreSnapshot: the Flow peer score snapshot.
func convertPeerScoreSnapshots(snapshot map[peer.ID]*corrupt.PeerScoreSnapshot) map[peer.ID]*p2p.PeerScoreSnapshot {
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
func convertTopicScoreSnapshot(snapshot map[string]*corrupt.TopicScoreSnapshot) map[string]*p2p.TopicScoreSnapshot {
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
