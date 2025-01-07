package data_providers

import (
	"context"

	"github.com/google/uuid"

	"github.com/onflow/flow-go/engine/access/subscription"
)

// BaseDataProvider holds common objects for the provider
type BaseDataProvider struct {
	id           uuid.UUID
	topic        string
	cancel       context.CancelFunc
	send         chan<- interface{}
	subscription subscription.Subscription
}

// newBaseDataProvider creates a new instance of BaseDataProvider.
func newBaseDataProvider(
	topic string,
	cancel context.CancelFunc,
	send chan<- interface{},
	subscription subscription.Subscription,
) *BaseDataProvider {
	return &BaseDataProvider{
		id:           uuid.New(),
		topic:        topic,
		cancel:       cancel,
		send:         send,
		subscription: subscription,
	}
}

// ID returns the unique identifier of the data provider.
func (b *BaseDataProvider) ID() uuid.UUID {
	return b.id
}

// Topic returns the topic associated with the data provider.
func (b *BaseDataProvider) Topic() string {
	return b.topic
}

// Close terminates the data provider.
//
// No errors are expected during normal operations.
func (b *BaseDataProvider) Close() {
	b.cancel()
}
