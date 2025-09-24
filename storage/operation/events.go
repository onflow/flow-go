package operation

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

func eventPrefix(prefix byte, blockID flow.Identifier, event flow.Event) []byte {
	return MakePrefix(prefix, blockID, event.TransactionID, event.TransactionIndex, event.EventIndex)
}

func InsertEvent(lctx lockctx.Proof, w storage.Writer, blockID flow.Identifier, event flow.Event) error {
	if !lctx.HoldsLock(storage.LockInsertOwnReceipt) {
		return fmt.Errorf("InsertEvent requires LockInsertOwnReceipt to be held")
	}
	return UpsertByKey(w, eventPrefix(codeEvent, blockID, event), event)
}

func InsertServiceEvent(lctx lockctx.Proof, w storage.Writer, blockID flow.Identifier, event flow.Event) error {
	if !lctx.HoldsLock(storage.LockInsertOwnReceipt) {
		return fmt.Errorf("InsertServiceEvent requires LockInsertOwnReceipt to be held")
	}
	return UpsertByKey(w, eventPrefix(codeServiceEvent, blockID, event), event)
}

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

func RemoveEventsByBlockID(r storage.Reader, w storage.Writer, blockID flow.Identifier) error {
	return RemoveByKeyPrefix(r, w, MakePrefix(codeEvent, blockID))
}

// eventIterationFunc returns an in iteration function which returns all events found during traversal or iteration
func eventIterationFunc(events *[]flow.Event) IterationFunc {
	return func(keyCopy []byte, getValue func(destVal any) error) (bail bool, err error) {
		var event flow.Event
		err = getValue(&event)
		if err != nil {
			return true, err
		}
		*events = append(*events, event)
		return false, nil
	}
}

// eventFilterIterationFunc returns an iteration function which filters the result by the given event type in the handleFunc
func eventFilterIterationFunc(events *[]flow.Event, eventType flow.EventType) IterationFunc {
	return func(keyCopy []byte, getValue func(destVal any) error) (bail bool, err error) {
		var event flow.Event
		err = getValue(&event)
		if err != nil {
			return true, err
		}
		// filter out all events not of type eventType
		if event.Type == eventType {
			*events = append(*events, event)
		}
		return false, nil
	}
}
