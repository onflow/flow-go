// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

func InsertEvent(blockID flow.Identifier, event flow.Event) func(*badger.Txn) error {
	return insert(makePrefix(codeEvent, blockID, event.TransactionID, event.TransactionIndex, event.EventIndex), event)
}

func RetrieveEvents(blockID flow.Identifier, transactionID flow.Identifier, events *[]flow.Event) func(*badger.Txn) error {
	iterationFunc := eventIterationFunc(events)
	return traverse(makePrefix(codeEvent, blockID, transactionID), iterationFunc)
}

func LookupEventsByBlockID(blockID flow.Identifier, events *[]flow.Event) func(*badger.Txn) error {
	iterationFunc := eventIterationFunc(events)
	return traverse(makePrefix(codeEvent, blockID), iterationFunc)
}

func LookupEventsByBlockIDEventType(blockID flow.Identifier, eventType flow.EventType, events *[]flow.Event) func(*badger.Txn) error {
	iterationFunc := eventFilterIterationFunc(events, eventType)
	return traverse(makePrefix(codeEvent, blockID), iterationFunc)
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
