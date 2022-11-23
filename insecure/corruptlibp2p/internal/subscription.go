package internal

import (
	"context"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	corrupt "github.com/yhassanzadeh13/go-libp2p-pubsub"

	"github.com/onflow/flow-go/network/p2p"
)

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
