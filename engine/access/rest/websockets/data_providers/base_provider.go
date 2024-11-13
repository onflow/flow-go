package data_providers

import (
	"context"

	"github.com/onflow/flow-go/engine/access/subscription"
)

// BaseDataProvider defines the basic interface for a data provider. It provides methods
// for retrieving the provider's unique ID, topic, and a method to close the provider.
type BaseDataProvider interface {
	// ID returns the unique identifier of subscription in the data provider.
	ID() string
	// Topic returns the topic associated with the data provider.
	Topic() string
	// Close terminates the data provider.
	Close()
}

var _ BaseDataProvider = (*BaseDataProviderImpl)(nil)

// BaseDataProviderImpl is the concrete implementation of the BaseDataProvider interface.
// It holds common objects for the provider.
type BaseDataProviderImpl struct {
	topic        string
	cancel       context.CancelFunc
	send         chan<- interface{}
	subscription subscription.Subscription
}

// NewBaseDataProviderImpl creates a new instance of BaseDataProviderImpl.
func NewBaseDataProviderImpl(
	cancel context.CancelFunc,
	topic string,
	send chan<- interface{},
	subscription subscription.Subscription,
) *BaseDataProviderImpl {
	return &BaseDataProviderImpl{
		topic:        topic,
		cancel:       cancel,
		send:         send,
		subscription: subscription,
	}
}

// ID returns the unique identifier of the data provider's subscription.
func (b *BaseDataProviderImpl) ID() string {
	return b.subscription.ID()
}

// Topic returns the topic associated with the data provider.
func (b *BaseDataProviderImpl) Topic() string {
	return b.topic
}

// Close terminates the data provider.
func (b *BaseDataProviderImpl) Close() {
	b.cancel()
}
