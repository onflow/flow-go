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
		closedChan:     make(chan struct{}),
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
	b.closedFlag.Do(func() {
		close(b.closedChan)
	})
}

type sendResponseCallback[T any] func(T) error

// run reads data from a subscription and sends it to clients using the provided
// sendResponse callback. It continuously listens to the subscription's data
// channel and forwards the received values until the closedChan is closed or
// the subscription ends. It is used as a helper function for each data provider's
// Run() function.
//
// Parameters:
//   - closedChan: A channel to signal the termination of the function. When closed,
//     the function stops reading from the subscription and exits gracefully.
//   - subscription: An instance of the Subscription interface, which provides a
//     data stream through its Channel() method and an optional error through Err().
//   - sendResponse: A callback function that processes and forwards the received
//     data to the clients (e.g. a WebSocket controller). If the callback
//     returns an error, the function terminates with that error.
//
// Returns:
//   - error: If any error occurs while reading from the subscription or sending
//     responses, it returns an error wrapped with additional context. If the
//     closedChan is closed or the subscription ends without errors, it returns nil.
//
// Errors:
//   - If the subscription ends with an error, it is wrapped and returned.
//   - If a received value is not of the expected type (T), an error is returned.
//   - If the sendResponse callback encounters an error, it is wrapped and returned.
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
