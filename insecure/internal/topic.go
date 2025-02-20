package internal

import (
	"context"

	corrupt "github.com/libp2p/go-libp2p-pubsub"

	"github.com/onflow/flow-go/network/p2p"
)

// CorruptTopic is a wrapper that implements the p2p.Topic.
// This is previously needed because we used a forked pubsub module. This is no longer the case
// so we could refactor this in the future to remove this wrapper.
type CorruptTopic struct {
	t *corrupt.Topic
}

var _ p2p.Topic = (*CorruptTopic)(nil)

func NewCorruptTopic(t *corrupt.Topic) p2p.Topic {
	return &CorruptTopic{
		t: t,
	}
}

func (c *CorruptTopic) String() string {
	return c.t.String()
}

func (c *CorruptTopic) Close() error {
	return c.t.Close()
}

func (c *CorruptTopic) Publish(ctx context.Context, bytes []byte) error {
	return c.t.Publish(ctx, bytes)
}

func (c *CorruptTopic) Subscribe() (p2p.Subscription, error) {
	sub, err := c.t.Subscribe()
	if err != nil {
		return nil, err
	}
	return NewCorruptSubscription(sub), nil
}
