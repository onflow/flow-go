// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/encoding"
)

// List of built-in account event types.
const (
	EventAccountCreated EventType = "flow.AccountCreated"
	EventAccountUpdated EventType = "flow.AccountUpdated"
)

type EventType string

type Event struct {
	// Type is the qualified event type.
	Type EventType
	// TransactionID is the ID of the transaction this event was emitted from.
	TransactionID Identifier
	// TransactionIndex defines the index of the transaction this event was emitted from within the block.
	// The first transaction has index 0, the second has index 1, and so on.
	TransactionIndex uint32
	// EventIndex defines the ordering of events in a transaction.
	// The first event emitted has index 0, the second has index 1, and so on.
	EventIndex uint32
	// Payload contains the encoded event data.
	Payload []byte
}

// String returns the string representation of this event.
func (e Event) String() string {
	return fmt.Sprintf("%s: %s", e.Type, e.ID())
}

// ID returns a canonical identifier that is guaranteed to be unique.
func (e Event) ID() Identifier {
	return MakeID(e.Body())
}

// Body returns the body of the execution receipt.
func (e *Event) Body() interface{} {
	return wrapEvent(*e)
}

// Encode returns the canonical encoding of the event, containing only the
// fields necessary to uniquely identify it.
func (e Event) Encode() []byte {
	w := wrapEvent(e)
	return encoding.DefaultEncoder.MustEncode(w)
}

// Defines only the fields needed to uniquely identify an event.
type eventWrapper struct {
	TxID  []byte
	Index uint32
}

func wrapEvent(e Event) eventWrapper {
	return eventWrapper{
		TxID:  e.TransactionID[:],
		Index: e.EventIndex,
	}
}
