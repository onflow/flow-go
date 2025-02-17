package data_providers

import (
	"context"
	"fmt"

	"go.uber.org/atomic"

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
	// Indicates whether the provider's Close() method has been called.
	// Used to distinguish between intentional closure and termination
	// due to a subscription error.
	// Note: This variable must be set only in Close() method.
	closedByClient atomic.Bool
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
		closedByClient: atomic.Bool{},
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
	b.closedByClient.Store(true)
	b.cancel()
}

// wasClosedByClient returns true if the provider was explicitly closed via the Close() method.
// Returns false if the provider was closed due to a subscription error or other factors.
func (b *baseDataProvider) wasClosedByClient() bool {
	return b.closedByClient.Load()
}

// handleSubscriptionError ensures proper error handling due to the way subscriptions work.
// The `subscription` package returns a `context.Canceled` error when the streamer's context is canceled.
// The DataProvider also uses a context to stop execution via the `Close()` method.
// As a result, when a subscription ends, a `context.Canceled` error can originate from two sources:
//  1. The streamer (subscription itself).
//  2. The caller of `Close()` (e.g., the WebSocket controller).
//
// To distinguish between these cases, we use the `wasClosedByClient()` method.
// If the provider was intentionally closed, we suppress the error; otherwise, we propagate it.
func (b *baseDataProvider) handleSubscriptionError(err error) error {
	if err != nil {
		if b.wasClosedByClient() {
			return nil
		}

		return fmt.Errorf("subscription was closed: %w", err)
	}

	return nil
}
