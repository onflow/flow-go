// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED
package flow

import (
	"fmt"
)

// TransactionError is the run time Cadence error that may be generated during the execution of a transaction
type TransactionError struct {
	// TransactionID is the ID of the transaction this error was emitted from.
	TransactionID Identifier
	// Message contains the error message
	Message string
}

// String returns the string representation of this error.
func (te TransactionError) String() string {
	return fmt.Sprintf("Transaction ID: %s, Error Message: %s", te.TransactionID.String(), te.Message)
}

// ID returns a canonical identifier that is guaranteed to be unique.
func (te TransactionError) ID() Identifier {
	return te.TransactionID
}

func (te *TransactionError) Checksum() Identifier {
	return te.ID()
}
