package data_providers

import (
	"context"

	wsmodels "github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/engine/access/subscription"
)

// baseDataProvider holds common objects for the provider
type baseDataProvider struct {
	subscriptionID string
	topic          string
	arguments      wsmodels.Arguments
	cancel         context.CancelFunc
	send           chan<- interface{}
	subscription   subscription.Subscription
}

// newBaseDataProvider creates a new instance of baseDataProvider.
func newBaseDataProvider(
	subscriptionID string,
	topic string,
	arguments wsmodels.Arguments,
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
	return b.arguments
}

// Close terminates the data provider.
//
// No errors are expected during normal operations.
func (b *baseDataProvider) Close() {
	b.cancel()
}
