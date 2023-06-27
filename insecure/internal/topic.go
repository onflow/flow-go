package internal

import (
	"context"

	corrupt "github.com/yhassanzadeh13/go-libp2p-pubsub"

	"github.com/onflow/flow-go/network/p2p"
)

// CorruptTopic is a wrapper around the forked pubsub topic from
// github.com/yhassanzadeh13/go-libp2p-pubsub that implements the p2p.Topic.
// This is needed because in order to use the forked pubsub module, we need to
// use the entire dependency tree of the forked module which is resolved to
// github.com/yhassanzadeh13/go-libp2p-pubsub. This means that we cannot use
// the original libp2p pubsub module in the same package.
// Note: we use the forked pubsub module for sake of BFT testing and attack vector
// implementation, it is designed to be completely isolated in the "insecure" package, and
// totally separated from the rest of the codebase.
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
