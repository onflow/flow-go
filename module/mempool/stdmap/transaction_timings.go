package stdmap

import (
	"github.com/onflow/flow-go/model/flow"
)

// TransactionTimings implements the transaction timings memory pool of access nodes,
// used to store transaction timings to report the timing of individual transactions
type TransactionTimings struct {
	*Backend[flow.Identifier, *flow.TransactionTiming]
}

// NewTransactionTimings creates a new memory pool for transaction timings
func NewTransactionTimings(limit uint) (*TransactionTimings, error) {
	t := &TransactionTimings{
		Backend: NewBackend(WithLimit[flow.Identifier, *flow.TransactionTiming](limit)),
	}

	return t, nil
}

// Add adds a transaction timing to the mempool.
func (t *TransactionTimings) Add(tx *flow.TransactionTiming) bool {
	return t.Backend.Add(tx.ID(), tx)
}

// ByID returns the transaction timing with the given ID from the mempool.
func (t *TransactionTimings) ByID(txID flow.Identifier) (*flow.TransactionTiming, bool) {
	tt, exists := t.Backend.Get(txID)
	if !exists {
		return nil, false
	}
	return tt, true
}

// Adjust will adjust the transaction timing using the given function if the given key can be found.
// Returns a bool which indicates whether the value was updated as well as the updated value.
func (t *TransactionTimings) Adjust(txID flow.Identifier, f func(*flow.TransactionTiming) *flow.TransactionTiming) (
	*flow.TransactionTiming, bool) {
	tt, updated := t.Backend.Adjust(txID, func(tt *flow.TransactionTiming) *flow.TransactionTiming {
		return f(tt)
	})
	if !updated {
		return nil, false
	}
	return tt, updated
}

// All returns all transaction timings from the mempool.
func (t *TransactionTimings) All() []*flow.TransactionTiming {
	all := t.Backend.All()
	txs := make([]*flow.TransactionTiming, 0, len(all))
	for _, tx := range all {
		txs = append(txs, tx)
	}
	return txs
}

// Remove removes the transaction timing with the given ID.
func (t *TransactionTimings) Remove(txID flow.Identifier) bool {
	return t.Backend.Remove(txID)
}
