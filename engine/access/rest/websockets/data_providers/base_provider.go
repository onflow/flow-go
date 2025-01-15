package data_providers

import (
	"context"

	"github.com/google/uuid"

	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/engine/access/subscription"
)

// baseDataProvider holds common objects for the provider
type baseDataProvider struct {
	id           uuid.UUID
	topic        string
	arguments    models.Arguments
	cancel       context.CancelFunc
	send         chan<- interface{}
	subscription subscription.Subscription
}

// newBaseDataProvider creates a new instance of baseDataProvider.
func newBaseDataProvider(
	topic string,
	arguments models.Arguments,
	cancel context.CancelFunc,
	send chan<- interface{},
	subscription subscription.Subscription,
) *baseDataProvider {
	return &baseDataProvider{
		id:           uuid.New(),
		topic:        topic,
		arguments:    arguments,
		cancel:       cancel,
		send:         send,
		subscription: subscription,
	}
}

// ID returns the unique identifier of the data provider.
func (b *baseDataProvider) ID() uuid.UUID {
	return b.id
}

// Topic returns the topic associated with the data provider.
func (b *baseDataProvider) Topic() string {
	return b.topic
}

// Arguments returns the arguments associated with the data provider.
func (b *baseDataProvider) Arguments() models.Arguments {
	return b.arguments
}

// Close terminates the data provider.
//
// No errors are expected during normal operations.
func (b *baseDataProvider) Close() {
	b.cancel()
}
