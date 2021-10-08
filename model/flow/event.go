// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

import (
	"fmt"
	"time"

	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/fingerprint"
)

// List of built-in event types.
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
	return MakeID(wrapEventID(e))
}

func (e Event) Checksum() Identifier {
	return MakeID(e)
}

// Encode returns the canonical encoding of this event, containing only the fields necessary to uniquely identify it.
func (e Event) Encode() []byte {
	w := wrapEventID(e)
	return encoding.DefaultEncoder.MustEncode(w)
}

func (e Event) Fingerprint() []byte {
	return fingerprint.Fingerprint(wrapEvent(e))
}

// Defines only the fields needed to uniquely identify an event.
type eventIDWrapper struct {
	TxID  []byte
	Index uint32
}

type eventWrapper struct {
	TxID             []byte
	Index            uint32
	Type             string
	TransactionIndex uint32
	Payload          []byte
}

func wrapEventID(e Event) eventIDWrapper {
	return eventIDWrapper{
		TxID:  e.TransactionID[:],
		Index: e.EventIndex,
	}
}

func wrapEvent(e Event) eventWrapper {
	return eventWrapper{
		TxID:             e.TransactionID[:],
		Index:            e.EventIndex,
		Type:             string(e.Type),
		TransactionIndex: e.TransactionIndex,
		Payload:          e.Payload[:],
	}
}

// BlockEvents contains events emitted in a single block.
type BlockEvents struct {
	BlockID        Identifier
	BlockHeight    uint64
	BlockTimestamp time.Time
	Events         []Event
}

type EventsList []Event

// EventsListHash calculates a hash of events list,
// by simply hashing byte representation of events in the lists
func EventsListHash(el EventsList) (Identifier, error) {

	hasher := hash.NewSHA3_256()

	for _, event := range el {
		_, err := hasher.Write(event.Fingerprint())
		if err != nil {
			return ZeroID, fmt.Errorf("cannot write to hasher: %w", err)
		}
	}

	return HashToID(hasher.SumHash()), nil
}
