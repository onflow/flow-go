package data_providers

import (
	"context"
	"fmt"
	"sync"

	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/engine/access/subscription"
)

// baseDataProvider holds common objects for the provider
type baseDataProvider struct {
	subscriptionID string
	topic          string
	arguments      models.Arguments
	cancel         context.CancelFunc
	send           chan<- interface{}
	subscription   subscription.Subscription
	// Ensures the closedChan has been closed once.
	closedFlag sync.Once
	closedChan chan struct{}
}

// newBaseDataProvider creates a new instance of baseDataProvider.
func newBaseDataProvider(
	subscriptionID string,
	topic string,
	arguments models.Arguments,
	cancel context.CancelFunc,
	send chan<- interface{},
	subscription subscription.Subscription,
) *baseDataProvider {
	return &baseDataProvider{
		subscriptionID: subscriptionID,
		topic:          topic,
		arguments:      arguments,
		cancel:         cancel,
		send:           send,
		subscription:   subscription,
		closedFlag:     sync.Once{},
		closedChan:     make(chan struct{}, 1),
	}
}

// ID returns the subscription ID associated with current data provider
func (b *baseDataProvider) ID() string {
	return b.subscriptionID
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

	//TODO: explain why we run it in a separate routine
	go func() {
		b.closedFlag.Do(func() {
			close(b.closedChan)
		})
	}()
}

type sendResponseCallback[T any] func(T) error

func run[T any](
	closedChan <-chan struct{},
	subscription subscription.Subscription,
	sendResponse sendResponseCallback[T],
) error {
	for {
		select {
		case <-closedChan:
			return nil
		case value, ok := <-subscription.Channel():
			if !ok {
				if subscription.Err() != nil {
					return fmt.Errorf("subscription finished with error: %w", subscription.Err())
				}
				return nil
			}

			response, ok := value.(T)
			if !ok {
				return fmt.Errorf("unexpected response type: %T", value)
			}

			err := sendResponse(response)
			if err != nil {
				return fmt.Errorf("error sending response: %w", err)
			}
		}
	}
}
