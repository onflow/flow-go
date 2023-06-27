package p2pnode

import (
	"context"

	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/onflow/flow-go/network/p2p"
)

// GossipSubTopic is a wrapper around libp2p pubsub topics that implements the
// PubSubTopic interface for the Flow network.
type GossipSubTopic struct {
	t *pubsub.Topic
}

var _ p2p.Topic = (*GossipSubTopic)(nil)

func NewGossipSubTopic(topic *pubsub.Topic) *GossipSubTopic {
	return &GossipSubTopic{
		t: topic,
	}
}

func (g *GossipSubTopic) String() string {
	return g.t.String()
}

func (g *GossipSubTopic) Close() error {
	return g.t.Close()
}

func (g *GossipSubTopic) Publish(ctx context.Context, bytes []byte) error {
	return g.t.Publish(ctx, bytes)
}

func (g *GossipSubTopic) Subscribe() (p2p.Subscription, error) {
	return g.t.Subscribe()
}
