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

func InsertServiceEvent(w storage.Writer, blockID flow.Identifier, event flow.Event) error {
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
	return func(unmarshal func(data []byte, v any) error) (CheckFunc, HandleFunc) {
		check := func(key []byte) (bool, error) {
			return true, nil
		}
		handle := func(data []byte) error {
			var val flow.Event
			err := unmarshal(data, &val)
			if err != nil {
				return err
			}
			*events = append(*events, val)
			return nil
		}
		return check, handle
	}
}

// eventFilterIterationFunc returns an iteration function which filters the result by the given event type in the handleFunc
func eventFilterIterationFunc(events *[]flow.Event, eventType flow.EventType) IterationFunc {
	return func(unmarshal func(data []byte, v any) error) (CheckFunc, HandleFunc) {
		check := func(key []byte) (bool, error) {
			return true, nil
		}
		handle := func(data []byte) error {
			var val flow.Event
			err := unmarshal(data, &val)
			if err != nil {
				return err
			}
			// filter out all events not of type eventType
			if val.Type == eventType {
				*events = append(*events, val)
			}
			return nil
		}
		return check, handle
	}
}
