package corruptlibp2p

import (
	"github.com/libp2p/go-libp2p/core/peer"
	corrupt "github.com/yhassanzadeh13/go-libp2p-pubsub"

	"github.com/onflow/flow-go/insecure/corruptlibp2p/internal"
	"github.com/onflow/flow-go/network/p2p"
)

type CorruptPubSubAdapter struct {
	gossipSub *corrupt.PubSub
}

var _ p2p.PubSubAdapter = (*CorruptPubSubAdapter)(nil)

func (c *CorruptPubSubAdapter) RegisterTopicValidator(topic string, val interface{}) error {
	return c.gossipSub.RegisterTopicValidator(topic, val, corrupt.WithValidatorInline(true))
}

func (c *CorruptPubSubAdapter) UnregisterTopicValidator(topic string) error {
	return c.gossipSub.UnregisterTopicValidator(topic)
}

func (c *CorruptPubSubAdapter) Join(topic string) (p2p.Topic, error) {
	t, err := c.gossipSub.Join(topic)
	if err != nil {
		return nil, err
	}
	return internal.NewCorruptTopic(t), nil
}

func (c *CorruptPubSubAdapter) GetTopics() []string {
	return c.gossipSub.GetTopics()
}

func (c *CorruptPubSubAdapter) ListPeers(topic string) []peer.ID {
	return c.ListPeers(topic)
}

func NewCorruptPubSubAdapter(gossipSub *corrupt.PubSub) p2p.PubSubAdapter {
	return &CorruptPubSubAdapter{
		gossipSub: gossipSub,
	}
}

var _ p2p.PubSubAdapter = (*CorruptPubSubAdapter)(nil)
