package internal

import (
	"context"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	corrupt "github.com/yhassanzadeh13/go-libp2p-pubsub"

	"github.com/onflow/flow-go/network/p2p"
)

// CorruptSubscription is a wrapper around the forked pubsub subscription from
// github.com/yhassanzadeh13/go-libp2p-pubsub that implements the p2p.Subscription.
// This is needed because in order to use the forked pubsub module, we need to
// use the entire dependency tree of the forked module which is resolved to
// github.com/yhassanzadeh13/go-libp2p-pubsub. This means that we cannot use
// the original libp2p pubsub module in the same package.
// Note: we use the forked pubsub module for sake of BFT testing and attack vector
// implementation, it is designed to be completely isolated in the "insecure" package, and
// totally separated from the rest of the codebase.
type CorruptSubscription struct {
	s *corrupt.Subscription
}

var _ p2p.Subscription = (*CorruptSubscription)(nil)

func NewCorruptSubscription(s *corrupt.Subscription) p2p.Subscription {
	return &CorruptSubscription{
		s: s,
	}
}

func (c *CorruptSubscription) Cancel() {
	c.s.Cancel()
}

func (c *CorruptSubscription) Next(ctx context.Context) (*pubsub.Message, error) {
	m, err := c.s.Next(ctx)
	if err != nil {
		return nil, err
	}
	return &pubsub.Message{
		Message:       m.Message,
		ID:            m.ID,
		ReceivedFrom:  m.ReceivedFrom,
		ValidatorData: m.ValidatorData,
		Local:         m.Local,
	}, nil
}

func (c *CorruptSubscription) Topic() string {
	return c.s.Topic()
}
