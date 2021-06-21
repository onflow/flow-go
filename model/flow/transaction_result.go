// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED
package flow

import (
	"fmt"
)

// TransactionResult contains the artifacts generated after executing a Cadence transaction.
type TransactionResult struct {
	// TransactionID is the ID of the transaction this error was emitted from.
	TransactionID Identifier
	// ErrorMessage contains the error message of any error that may have occurred when the transaction was executed
	ErrorMessage string
	// Computation used
	ComputationUsed uint64
}

// String returns the string representation of this error.
func (t TransactionResult) String() string {
	return fmt.Sprintf("Transaction ID: %s, Error Message: %s", t.TransactionID.String(), t.ErrorMessage)
}

// ID returns a canonical identifier that is guaranteed to be unique.
func (t TransactionResult) ID() Identifier {
	return t.TransactionID
}

func (te *TransactionResult) Checksum() Identifier {
	return te.ID()
}
