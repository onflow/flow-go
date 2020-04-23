// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED
package flow

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/encoding"
)

// TransactionError is the run time Cadence error that may be generated during the execution of a transaction
type TransactionError struct {
	// TransactionID is the ID of the transaction this event was emitted from.
	TransactionID Identifier
	// TransactionIndex defines the index of the transaction this event was emitted from within the block.
	// The first transaction has index 0, the second has index 1, and so on.
	TransactionIndex uint32
	// Message contains the error message
	Message string
}

// String returns the string representation of this event.
func (te TransactionError) String() string {
	return fmt.Sprintf("Transaction ID: %s, Transaction Index: %d, Error Message: %s",
		te.TransactionID.String(), te.TransactionIndex, te.Message)
}

// ID returns a canonical identifier that is guaranteed to be unique.
func (te TransactionError) ID() Identifier {
	return MakeID(te.Body())
}

// Body returns the body of the execution receipt.
func (te *TransactionError) Body() interface{} {
	return wrapEvent(*te)
}

// Encode returns the canonical encoding of the event, containing only the
// fields necessary to uniquely identify it.
func (te TransactionError) Encode() []byte {
	w := wrapEvent(te)
	return encoding.DefaultEncoder.MustEncode(w)
}

// Defines only the fields needed to uniquely identify an event.
type errorWrapper struct {
	TxID    []byte
	TxIndex uint32
}

func wrapError(te TransactionError) eventWrapper {
	return eventWrapper{
		TxID:    te.TransactionID[:],
		TxIndex: te.TransactionIndex,
	}
}
