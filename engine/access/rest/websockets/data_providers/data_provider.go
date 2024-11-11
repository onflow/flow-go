package data_providers

// The DataProvider is the interface abstracts of the actual subscriptions used by the WebSocketCollector.
type DataProvider interface {
	BaseDataProvider

	// Run starts processing the subscription and handles responses.
	//
	// No errors are expected during normal operations.
	Run() error
}
