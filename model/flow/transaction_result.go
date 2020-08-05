// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED
package flow

import (
	"fmt"
)

// TransactionResult contains the artifacts generated when executing a cadence transaction.
type TransactionResult struct {
	// TransactionID is the ID of the transaction this error was emitted from.
	TransactionID Identifier
	// ErrorMessage contains the error message of any error that may have occurred when the transaction was executed
	ErrorMessage string
}

// String returns the string representation of this error.
func (te TransactionResult) String() string {
	return fmt.Sprintf("Transaction ID: %s, Error Message: %s", te.TransactionID.String(), te.ErrorMessage)
}

// ID returns a canonical identifier that is guaranteed to be unique.
func (te TransactionResult) ID() Identifier {
	return te.TransactionID
}

func (te *TransactionResult) Checksum() Identifier {
	return te.ID()
}
