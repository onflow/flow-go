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
	// Memory used (estimation)
	MemoryUsed uint64
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

// TODO(ramtin): add canonical encoding and ID
type TransactionResults []TransactionResult

// LightTransactionResult represents a TransactionResult, omitting any fields that are prone to
// non-determinism; i.e. the error message and memory used estimate.
//
// While the net causes of a transaction failing are deterministic, the specific error and message
// propagated back to FVM are prone to bugs resulting in slight variations. Rather than including
// the error and risking execution forks if an undetected bug is introduced, we simplify it to just
// a boolean value. This will likely change in the future to include some additional information
// about the error.
//
// Additionally, MemoryUsed is omitted because it is an estimate from the specific execution node,
// and will vary across nodes.
type LightTransactionResult struct {
	// TransactionID is the ID of the transaction this result was emitted from.
	TransactionID Identifier
	// Failed is true if the transaction's execution failed resulting in an error, false otherwise.
	Failed bool
	// ComputationUsed is amount of computation used while executing the transaction.
	ComputationUsed uint64
}
