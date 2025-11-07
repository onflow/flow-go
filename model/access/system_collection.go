package access

import (
	"github.com/onflow/flow-go/model/flow"
)

// EventProvider is a callback function that returns a list of events required to construct the
// system collection.
type EventProvider func() (flow.EventsList, error)

// SystemCollectionBuilder defines the builders for the system collections and their transactions.
type SystemCollectionBuilder interface {
	// ProcessCallbacksTransaction constructs a transaction for processing callbacks, for the given callback.
	//
	// No error returns are expected during normal operation.
	ProcessCallbacksTransaction(chain flow.Chain) (*flow.TransactionBody, error)

	// ExecuteCallbacksTransactions constructs a list of transaction to execute callbacks, for the given chain.
	//
	// No error returns are expected during normal operation.
	ExecuteCallbacksTransactions(chain flow.Chain, processEvents flow.EventsList) ([]*flow.TransactionBody, error)

	// SystemChunkTransaction creates and returns the transaction corresponding to the
	// system chunk for the given chain.
	//
	// No error returns are expected during normal operation.
	SystemChunkTransaction(chain flow.Chain) (*flow.TransactionBody, error)

	// SystemCollection constructs a system collection for the given chain.
	// Uses the provided event provider to get events required to construct the system collection.
	// A nil event provider behaves the same as an event provider that returns an empty EventsList.
	//
	// No error returns are expected during normal operation.
	SystemCollection(chain flow.Chain, providerFn EventProvider) (*flow.Collection, error)
}

// StaticEventProvider returns an event provider that returns the given events.
func StaticEventProvider(events flow.EventsList) EventProvider {
	return func() (flow.EventsList, error) {
		return events, nil
	}
}
