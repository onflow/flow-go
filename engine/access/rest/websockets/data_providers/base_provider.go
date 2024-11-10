package data_providers

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/access"
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
	topic string

	cancel context.CancelFunc
	ctx    context.Context

	api          access.API
	send         chan<- interface{}
	subscription subscription.Subscription
}

// NewBaseDataProviderImpl creates a new instance of BaseDataProviderImpl.
func NewBaseDataProviderImpl(
	ctx context.Context,
	cancel context.CancelFunc,
	api access.API,
	topic string,
	send chan<- interface{},
	subscription subscription.Subscription,
) *BaseDataProviderImpl {
	return &BaseDataProviderImpl{
		topic: topic,

		ctx:    ctx,
		cancel: cancel,

		api:          api,
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

// TODO: refactor rpc version of HandleSubscription and use it
func HandleSubscription[T any](ctx context.Context, sub subscription.Subscription, handleResponse func(resp T) error) error {
	for {
		select {
		case v, ok := <-sub.Channel():
			if !ok {
				if sub.Err() != nil {
					return fmt.Errorf("stream encountered an error: %w", sub.Err())
				}
				return nil
			}

			resp, ok := v.(T)
			if !ok {
				return fmt.Errorf("unexpected subscription response type: %T", v)
			}

			err := handleResponse(resp)
			if err != nil {
				return err
			}
		case <-ctx.Done():
			// context closed, subscription closed
			return nil
		}
	}
}
