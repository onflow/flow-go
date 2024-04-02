package stdmap

import (
	"github.com/onflow/flow-go/model/flow"
)

// TransactionTimings implements the transaction timings memory pool of access nodes,
// used to store transaction timings to report the timing of individual transactions
type TransactionTimings struct {
	*Backend
}

// NewTransactionTimings creates a new memory pool for transaction timings
func NewTransactionTimings(limit uint) (*TransactionTimings, error) {
	t := &TransactionTimings{
		Backend: NewBackend(WithLimit(limit)),
	}

	return t, nil
}

// Add adds a transaction timing to the mempool.
func (t *TransactionTimings) Add(tx *flow.TransactionTiming) bool {
	return t.Backend.Add(tx)
}

// ByID returns the transaction timing with the given ID from the mempool.
func (t *TransactionTimings) ByID(txID flow.Identifier) (*flow.TransactionTiming, bool) {
	entity, exists := t.Backend.ByID(txID)
	if !exists {
		return nil, false
	}
	tt, ok := entity.(*flow.TransactionTiming)
	if !ok {
		return nil, false
	}
	return tt, true
}

// Adjust will adjust the transaction timing using the given function if the given key can be found.
// Returns a bool which indicates whether the value was updated as well as the updated value.
func (t *TransactionTimings) Adjust(txID flow.Identifier, f func(*flow.TransactionTiming) *flow.TransactionTiming) (
	*flow.TransactionTiming, bool) {
	e, updated := t.Backend.Adjust(txID, func(e flow.Entity) flow.Entity {
		tt, ok := e.(*flow.TransactionTiming)
		if !ok {
			return nil
		}
		return f(tt)
	})
	if !updated {
		return nil, false
	}
	tt, ok := e.(*flow.TransactionTiming)
	if !ok {
		return nil, false
	}
	return tt, updated
}

// All returns all transaction timings from the mempool.
func (t *TransactionTimings) All() []*flow.TransactionTiming {
	entities := t.Backend.All()
	txs := make([]*flow.TransactionTiming, 0, len(entities))
	for _, entity := range entities {
		txs = append(txs, entity.(*flow.TransactionTiming))
	}
	return txs
}

// Remove removes the transaction timing with the given ID.
func (t *TransactionTimings) Remove(txID flow.Identifier) bool {
	return t.Backend.Remove(txID)
}
