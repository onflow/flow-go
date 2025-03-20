package data_providers

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	wsmodels "github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/engine/access/subscription"
)

// baseDataProvider holds common objects for the provider
type baseDataProvider struct {
	logger            zerolog.Logger
	api               access.API
	subscriptionID    string
	topic             string
	rawArguments      wsmodels.Arguments
	send              chan<- interface{}
	subscriptionState *subscriptionState
}

type subscriptionState struct {
	cancelSubscriptionContext context.CancelFunc
	subscription              subscription.Subscription
}

func newSubscriptionState(cancel context.CancelFunc, subscription subscription.Subscription) *subscriptionState {
	return &subscriptionState{
		cancelSubscriptionContext: cancel,
		subscription:              subscription,
	}
}

// newBaseDataProvider creates a new instance of baseDataProvider.
func newBaseDataProvider(
	logger zerolog.Logger,
	api access.API,
	subscriptionID string,
	topic string,
	rawArguments wsmodels.Arguments,
	send chan<- interface{},
) *baseDataProvider {
	return &baseDataProvider{
		logger:            logger,
		api:               api,
		subscriptionID:    subscriptionID,
		topic:             topic,
		rawArguments:      rawArguments,
		send:              send,
		subscriptionState: nil,
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
func (b *baseDataProvider) Arguments() wsmodels.Arguments {
	return b.rawArguments
}

// Close terminates the data provider.
func (b *baseDataProvider) Close() {
	b.subscriptionState.cancelSubscriptionContext()
}

type sendResponseCallback[T any] func(T) error

// run reads data from a subscription and sends it to clients using the provided
// sendResponse callback. It continuously listens to the subscription's data
// channel and forwards the received values until the subscription ends.
// It is used as a helper function for each data provider's Run() function.
//
// Parameters:
//   - subscription: An instance of the Subscription interface, which provides a
//     data stream through its Channel() method and an optional error through Err().
//   - sendResponse: A callback function that processes and forwards the received
//     data to the clients (e.g. a WebSocket controller). If the callback
//     returns an error, the function terminates with that error.
//
// Returns:
//   - error: If any error occurs while reading from the subscription or sending
//     responses, it returns an error wrapped with additional context.
//
// Errors
//   - If the subscription or sendResponse return an error, it is returned.
//
// No other errors are expected during normal operation
func run[T any](
	subscription subscription.Subscription,
	sendResponse sendResponseCallback[T],
) error {
	for {
		value, ok := <-subscription.Channel()
		if !ok {
			err := subscription.Err()
			if err != nil && !errors.Is(err, context.Canceled) {
				return fmt.Errorf("subscription finished with error: %w", err)
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
