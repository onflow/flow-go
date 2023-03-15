package result

import "github.com/onflow/flow-go/model/flow"

// ExecutedCollection holds results of a collection execution
type ExecutedCollection interface {
	// BlockHeader returns the block header in which collection was included
	BlockHeader() *flow.Header

	// Collection returns the content of the collection
	Collection() *flow.Collection

	// RegisterUpdates returns all registers that were updated during collection execution
	UpdatedRegisters() flow.RegisterEntries

	// TouchedRegisters returns all registers that has been touched (read/written) during collection execution
	TouchedRegisters() flow.RegisterIDs

	// EmittedEvents returns a list of events emitted during collection execution
	EmittedEvents() flow.EventsList

	// TransactionResults returns a list of transaction results
	TransactionResults() flow.TransactionResults
}

// ExecutedCollectionConsumer consumes ExecutedCollections
type ExecutedCollectionConsumer interface {
	OnExecutedCollection(ec ExecutedCollection) error
}
