package operation

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

func eventPrefix(prefix byte, blockID flow.Identifier, event flow.Event) []byte {
	return MakePrefix(prefix, blockID, event.TransactionID, event.TransactionIndex, event.EventIndex)
}

func InsertEvent(w storage.Writer, blockID flow.Identifier, event flow.Event) error {
	return UpsertByKey(w, eventPrefix(codeEvent, blockID, event), event)
}

//TODO: remove commented functions

//	func BatchInsertEvent(blockID flow.Identifier, event flow.Event) func(batch *badger.WriteBatch) error {
//		return batchWrite(eventPrefix(codeEvent, blockID, event), event)
//	}
func InsertServiceEvent(w storage.Writer, blockID flow.Identifier, event flow.Event) error {
	return UpsertByKey(w, eventPrefix(codeServiceEvent, blockID, event), event)
}

//	func BatchInsertServiceEvent(blockID flow.Identifier, event flow.Event) func(batch *badger.WriteBatch) error {
//		return batchWrite(eventPrefix(codeServiceEvent, blockID, event), event)
//	}
func RetrieveEvents(r storage.Reader, blockID flow.Identifier, transactionID flow.Identifier, events *[]flow.Event) error {
	iterationFunc := eventIterationFunc(events)
	return TraverseByPrefix(r, MakePrefix(codeEvent, blockID, transactionID), iterationFunc, storage.DefaultIteratorOptions())
}

func LookupEventsByBlockID(r storage.Reader, blockID flow.Identifier, events *[]flow.Event) error {
	iterationFunc := eventIterationFunc(events)
	return TraverseByPrefix(r, MakePrefix(codeEvent, blockID), iterationFunc, storage.DefaultIteratorOptions())
}

func LookupServiceEventsByBlockID(r storage.Reader, blockID flow.Identifier, events *[]flow.Event) error {
	iterationFunc := eventIterationFunc(events)
	return TraverseByPrefix(r, MakePrefix(codeServiceEvent, blockID), iterationFunc, storage.DefaultIteratorOptions())
}

func LookupEventsByBlockIDEventType(r storage.Reader, blockID flow.Identifier, eventType flow.EventType, events *[]flow.Event) error {
	iterationFunc := eventFilterIterationFunc(events, eventType)
	return TraverseByPrefix(r, MakePrefix(codeEvent, blockID), iterationFunc, storage.DefaultIteratorOptions())
}

func RemoveServiceEventsByBlockID(r storage.Reader, w storage.Writer, blockID flow.Identifier) error {
	return RemoveByKeyPrefix(r, w, MakePrefix(codeServiceEvent, blockID))
}

// // BatchRemoveServiceEventsByBlockID removes all service events for the given blockID.
// // No errors are expected during normal operation, even if no entries are matched.
// // If Badger unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
// func BatchRemoveServiceEventsByBlockID(blockID flow.Identifier, batch *badger.WriteBatch) func(*badger.Txn) error {
// 	return func(txn *badger.Txn) error {
// 		return batchRemoveByPrefix(makePrefix(codeServiceEvent, blockID))(txn, batch)
// 	}
// }

func RemoveEventsByBlockID(r storage.Reader, w storage.Writer, blockID flow.Identifier) error {
	return RemoveByKeyPrefix(r, w, MakePrefix(codeEvent, blockID))
}

// // BatchRemoveEventsByBlockID removes all events for the given blockID.
// // No errors are expected during normal operation, even if no entries are matched.
// // If Badger unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
// func BatchRemoveEventsByBlockID(blockID flow.Identifier, batch *badger.WriteBatch) func(*badger.Txn) error {
// 	return func(txn *badger.Txn) error {
// 		return batchRemoveByPrefix(makePrefix(codeEvent, blockID))(txn, batch)
// 	}
//
// }

// eventIterationFunc returns an in iteration function which returns all events found during traversal or iteration
func eventIterationFunc(events *[]flow.Event) func() (CheckFunc, CreateFunc, HandleFunc) {
	return func() (CheckFunc, CreateFunc, HandleFunc) {
		check := func(key []byte) (bool, error) {
			return true, nil
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
func eventFilterIterationFunc(events *[]flow.Event, eventType flow.EventType) func() (CheckFunc, CreateFunc, HandleFunc) {
	return func() (CheckFunc, CreateFunc, HandleFunc) {
		check := func(key []byte) (bool, error) {
			return true, nil
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
