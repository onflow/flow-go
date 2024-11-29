package data_providers

import (
	"github.com/google/uuid"
)

// The DataProvider is the interface abstracts of the actual data provider used by the WebSocketCollector.
// It provides methods for retrieving the provider's unique ID, topic, and a methods to close and run the provider.
type DataProvider interface {
	// ID returns the unique identifier of the data provider.
	ID() uuid.UUID
	// Topic returns the topic associated with the data provider.
	Topic() string
	// Close terminates the data provider.
	//
	// No errors are expected during normal operations.
	Close() error
	// Run starts processing the subscription and handles responses.
	//
	// No errors are expected during normal operations.
	Run() error
}
