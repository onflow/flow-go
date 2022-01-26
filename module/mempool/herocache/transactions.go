package herocache

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
	"github.com/onflow/flow-go/module/mempool/stdmap"
)

type Transactions struct {
	c *stdmap.Backend
}

// NewTransactions implements a transactions mempool based on hero cache.
func NewTransactions(limit uint32, logger zerolog.Logger) *Transactions {
	t := &Transactions{
		c: stdmap.NewBackendWithBackData(herocache.NewCache(limit,
			herocache.DefaultOversizeFactor,
			heropool.LRUEjection,
			logger.With().Str("mempool", "transactions").Logger())),
	}

	return t
}

// Has checks whether the transaction with the given hash is currently in
// the memory pool.
func (t Transactions) Has(id flow.Identifier) bool {
	return t.c.Has(id)
}

// Add adds a transaction to the mempool.
func (t *Transactions) Add(tx *flow.TransactionBody) bool {
	// Warning! reference pointer must be dereferenced before adding to HeroCache.
	// This is crucial for its heap object optimizations.
	return t.c.Add(*tx)
}

// ByID returns the transaction with the given ID from the mempool.
func (t Transactions) ByID(txID flow.Identifier) (*flow.TransactionBody, bool) {
	entity, exists := t.c.ByID(txID)
	if !exists {
		return nil, false
	}
	tx, ok := entity.(flow.TransactionBody)
	if !ok {
		panic(fmt.Sprintf("invalid entity in transaction pool (%T)", entity))
	}
	return &tx, true
}

// All returns all transactions from the mempool. Since it is using the HeroCache, all guarantees returning
// all transactions in the same order as they are added.
func (t Transactions) All() []*flow.TransactionBody {
	entities := t.c.All()
	txs := make([]*flow.TransactionBody, 0, len(entities))
	for _, entity := range entities {
		tx, ok := entity.(flow.TransactionBody)
		if !ok {
			panic(fmt.Sprintf("invalid entity in transaction pool (%T)", entity))
		}
		txs = append(txs, &tx)
	}
	return txs
}

// Clear removes all transactions stored in this mempool.
func (t *Transactions) Clear() {
	t.c.Clear()
}

// Size returns total number of stored transactions.
func (t Transactions) Size() uint {
	return t.c.Size()
}

// Rem removes transaction from mempool.
func (t *Transactions) Rem(id flow.Identifier) bool {
	return t.c.Rem(id)
}

// Hash will return a fingerprint hash representing the contents of the
// entire memory pool.
func (t Transactions) Hash() flow.Identifier {
	return t.c.Hash()
}
