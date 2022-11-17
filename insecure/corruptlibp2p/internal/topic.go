package internal

import (
	"context"

	corrupt "github.com/yhassanzadeh13/go-libp2p-pubsub"

	"github.com/onflow/flow-go/network/p2p"
)

type CorruptTopic struct {
	t *corrupt.Topic
}

func (c *CorruptTopic) Subscribe() (p2p.Subscription, error) {
	sub, err := c.t.Subscribe()
	if err != nil {
		return nil, err
	}
	return NewCorruptSubscription(sub), nil
}

func NewCorruptTopic(t *corrupt.Topic) p2p.Topic {
	return &CorruptTopic{
		t: t,
	}
}

var _ p2p.Topic = (*CorruptTopic)(nil)

func (c *CorruptTopic) String() string {
	return c.t.String()
}

func (c *CorruptTopic) Close() error {
	return c.t.Close()
}

func (c *CorruptTopic) Publish(ctx context.Context, bytes []byte) error {
	return c.t.Publish(ctx, bytes)
}
