package p2pnode

import (
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	discoveryrouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"

	"github.com/onflow/flow-go/network/p2p"
)

// GossipSubAdapterConfig is a wrapper around libp2p pubsub options that
// implements the PubSubAdapterConfig interface for the Flow network.
type GossipSubAdapterConfig struct {
	options []pubsub.Option
}

var _ p2p.PubSubAdapterConfig = (*GossipSubAdapterConfig)(nil)

func NewGossipSubAdapterConfig(base *p2p.BasePubSubAdapterConfig) *GossipSubAdapterConfig {
	return &GossipSubAdapterConfig{
		options: defaultPubsubOptions(base),
	}
}

func (g *GossipSubAdapterConfig) WithRoutingDiscovery(routing routing.ContentRouting) {
	g.options = append(g.options, pubsub.WithDiscovery(discoveryrouting.NewRoutingDiscovery(routing)))
}

func (g *GossipSubAdapterConfig) WithSubscriptionFilter(filter p2p.SubscriptionFilter) {
	g.options = append(g.options, pubsub.WithSubscriptionFilter(filter))
}

func (g *GossipSubAdapterConfig) WithScoreOption(option p2p.ScoreOptionBuilder) {
	g.options = append(g.options, option.BuildFlowPubSubScoreOption())
}

func (g *GossipSubAdapterConfig) WithMessageIdFunction(f func([]byte) string) {
	g.options = append(g.options, pubsub.WithMessageIdFn(func(pmsg *pb.Message) string {
		return f(pmsg.Data)
	}))
}

func (g *GossipSubAdapterConfig) WithAppSpecificRpcInspector(f func(peer.ID, *pubsub.RPC) error) {
	g.options = append(g.options, pubsub.WithAppSpecificRpcInspector(f))
}

func (g *GossipSubAdapterConfig) WithScoreTracer(tracer p2p.PeerScoreTracer) {
	g.options = append(g.options, pubsub.WithPeerScoreInspect(func(snapshot map[peer.ID]*pubsub.PeerScoreSnapshot) {
		tracer.UpdatePeerScoreSnapshots(convertPeerScoreSnapshots(snapshot))
	}, tracer.UpdateInterval()))
}

// convertPeerScoreSnapshots converts a libp2p pubsub peer score snapshot to a Flow peer score snapshot.
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

func (g *GossipSubAdapterConfig) WithTracer(tracer p2p.PubSubTracer) {
	g.options = append(g.options, pubsub.WithRawTracer(tracer))
}

func (g *GossipSubAdapterConfig) Build() []pubsub.Option {
	return g.options
}

func defaultPubsubOptions(base *p2p.BasePubSubAdapterConfig) []pubsub.Option {
	return []pubsub.Option{
		pubsub.WithMessageSigning(true),
		pubsub.WithStrictSignatureVerification(true),
		pubsub.WithMaxMessageSize(base.MaxMessageSize),
	}
}
