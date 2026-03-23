package access

import (
	"github.com/onflow/flow-go/model/flow"
)

// TransactionResult represents a flow.TransactionResult with additional fields required for the Access API
type TransactionResult struct {
	Status          flow.TransactionStatus
	StatusCode      uint
	Events          []flow.Event
	ErrorMessage    string
	BlockID         flow.Identifier
	TransactionID   flow.Identifier
	CollectionID    flow.Identifier
	BlockHeight     uint64
	ComputationUsed uint64
}

func (r *TransactionResult) IsExecuted() bool {
	return r.Status == flow.TransactionStatusExecuted || r.Status == flow.TransactionStatusSealed
}

func (r *TransactionResult) IsFinal() bool {
	return r.Status == flow.TransactionStatusSealed || r.Status == flow.TransactionStatusExpired
}
