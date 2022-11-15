package p2p

import (
	"context"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
)

type PubSubAdapter interface {
	RegisterTopicValidator(topic string, val interface{}) error
	UnregisterTopicValidator(topic string) error
	Join(topic string) (Topic, error)
	GetTopics() []string
	ListPeers(topic string) []peer.ID
}

type Topic interface {
	String() string
	Close() error
	Publish(context.Context, []byte) error
	Subscribe() (Subscription, error)
}

type Subscription interface {
	Cancel()
	Next(context.Context) (*pubsub.Message, error)
}

type Message struct {
}
