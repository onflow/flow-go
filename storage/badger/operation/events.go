package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
)

func eventPrefix(prefix byte, blockID flow.Identifier, event flow.Event) []byte {
	return makePrefix(prefix, blockID, event.TransactionID, event.TransactionIndex, event.EventIndex)
}

func InsertEvent(blockID flow.Identifier, event flow.Event) func(*badger.Txn) error {
	return insert(eventPrefix(codeEvent, blockID, event), event)
}

func BatchInsertEvent(blockID flow.Identifier, event flow.Event) func(batch *badger.WriteBatch) error {
	return batchWrite(eventPrefix(codeEvent, blockID, event), event)
}

func InsertServiceEvent(blockID flow.Identifier, event flow.Event) func(*badger.Txn) error {
	return insert(eventPrefix(codeServiceEvent, blockID, event), event)
}

func BatchInsertServiceEvent(blockID flow.Identifier, event flow.Event) func(batch *badger.WriteBatch) error {
	return batchWrite(eventPrefix(codeServiceEvent, blockID, event), event)
}

func RetrieveEvents(blockID flow.Identifier, transactionID flow.Identifier, events *[]flow.Event) func(*badger.Txn) error {
	iterationFunc := eventIterationFunc(events)
	return traverse(makePrefix(codeEvent, blockID, transactionID), iterationFunc)
}

func LookupEventsByBlockID(blockID flow.Identifier, events *[]flow.Event) func(*badger.Txn) error {
	iterationFunc := eventIterationFunc(events)
	return traverse(makePrefix(codeEvent, blockID), iterationFunc)
}

func LookupServiceEventsByBlockID(blockID flow.Identifier, events *[]flow.Event) func(*badger.Txn) error {
	iterationFunc := eventIterationFunc(events)
	return traverse(makePrefix(codeServiceEvent, blockID), iterationFunc)
}

func LookupEventsByBlockIDEventType(blockID flow.Identifier, eventType flow.EventType, events *[]flow.Event) func(*badger.Txn) error {
	iterationFunc := eventFilterIterationFunc(events, eventType)
	return traverse(makePrefix(codeEvent, blockID), iterationFunc)
}

func RemoveServiceEventsByBlockID(blockID flow.Identifier) func(*badger.Txn) error {
	return removeByPrefix(makePrefix(codeServiceEvent, blockID))
}

// BatchRemoveServiceEventsByBlockID removes all service events for the given blockID.
// No errors are expected during normal operation, even if no entries are matched.
// If Badger unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
func BatchRemoveServiceEventsByBlockID(blockID flow.Identifier, batch *badger.WriteBatch) func(*badger.Txn) error {
	return func(txn *badger.Txn) error {
		return batchRemoveByPrefix(makePrefix(codeServiceEvent, blockID))(txn, batch)
	}
}

func RemoveEventsByBlockID(blockID flow.Identifier) func(*badger.Txn) error {
	return removeByPrefix(makePrefix(codeEvent, blockID))
}

// BatchRemoveEventsByBlockID removes all events for the given blockID.
// No errors are expected during normal operation, even if no entries are matched.
// If Badger unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
func BatchRemoveEventsByBlockID(blockID flow.Identifier, batch *badger.WriteBatch) func(*badger.Txn) error {
	return func(txn *badger.Txn) error {
		return batchRemoveByPrefix(makePrefix(codeEvent, blockID))(txn, batch)
	}

}

// eventIterationFunc returns an in iteration function which returns all events found during traversal or iteration
func eventIterationFunc(events *[]flow.Event) func() (checkFunc, createFunc, handleFunc) {
	return func() (checkFunc, createFunc, handleFunc) {
		check := func(key []byte) bool {
			return true
		}
		var val flow.Event
		create := func() interface{} {
			return &val
		}
		handle := func() error {
			*events = append(*events, val)
			return nil
		}
		return check, create, handle
	}
}

// eventFilterIterationFunc returns an iteration function which filters the result by the given event type in the handleFunc
func eventFilterIterationFunc(events *[]flow.Event, eventType flow.EventType) func() (checkFunc, createFunc, handleFunc) {
	return func() (checkFunc, createFunc, handleFunc) {
		check := func(key []byte) bool {
			return true
		}
		var val flow.Event
		create := func() interface{} {
			return &val
		}
		handle := func() error {
			// filter out all events not of type eventType
			if val.Type == eventType {
				*events = append(*events, val)
			}
			return nil
		}
		return check, create, handle
	}
}
