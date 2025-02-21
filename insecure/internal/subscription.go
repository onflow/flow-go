package internal

import (
	"context"

	corrupt "github.com/libp2p/go-libp2p-pubsub"
	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/onflow/flow-go/network/p2p"
)

// CorruptSubscription is a wrapper that implements the p2p.Subscription.
// This is previously needed because we used a forked pubsub module. This is no longer the case
// so we could refactor this in the future to remove this wrapper.
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

	// we read a corrupt.Message from the corrupt.Subscription, however, we need to return
	// a pubsub.Message to the caller of this function, so we need to convert the corrupt.Message.
	// Flow codebase uses the original libp2p pubsub module, and the pubsub.Message is defined
	// in the original libp2p pubsub module, so we cannot use the corrupt.Message in the Flow codebase.
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
