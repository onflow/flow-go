package data_providers

import (
	"github.com/google/uuid"
)

// The DataProvider is the interface abstracts of the actual data provider used by the WebSocketCollector.
// It provides methods for retrieving the provider's unique SubscriptionID, topic, and a methods to close and run the provider.
type DataProvider interface {
	// ID returns the unique identifier of the data provider.
	ID() uuid.UUID
	// Topic returns the topic associated with the data provider.
	Topic() string
	// Close terminates the data provider.
	//
	// No errors are expected during normal operations.
	Close()
	// Run starts processing the subscription and handles responses.
	//
	// The separation of the data provider's creation and its Run() method
	// allows for better control over the subscription lifecycle. By doing so,
	// a confirmation message can be sent to the client immediately upon
	// successful subscription creation or failure. This ensures any required
	// setup or preparation steps can be handled prior to initiating the
	// subscription and data streaming process.
	//
	// Run() begins the actual processing of the subscription. At this point,
	// the context used for provider creation is no longer needed, as all
	// necessary preparation steps should have been completed.
	//
	// No errors are expected during normal operations.
	Run() error
}
