package stdmap

import (
	"github.com/onflow/flow-go/model/flow"
)

// TransactionTimings implements the transaction timings memory pool of access nodes,
// used to store transaction timings to report the timing of individual transactions.
// Stored TransactionTiming are keyed by transaction ID.
type TransactionTimings struct {
	*Backend[flow.Identifier, *flow.TransactionTiming]
}

// NewTransactionTimings creates a new memory pool for transaction timings.
func NewTransactionTimings(limit uint) (*TransactionTimings, error) {
	return &TransactionTimings{NewBackend(WithLimit[flow.Identifier, *flow.TransactionTiming](limit))}, nil
}
