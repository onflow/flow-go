package herocache

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
)

const DefaultOversizeFactor = uint32(8)

type Transactions struct {
	*herocache.Cache
}

// NewTransactions implements a transactions mempool based on hero cache.
func NewTransactions(limit uint32, logger zerolog.Logger) *Transactions {
	t := &Transactions{
		Cache: herocache.NewCache(limit, DefaultOversizeFactor, heropool.LRUEjection, logger),
	}

	return t
}

// Add adds a transaction to the mempool.
func (t *Transactions) Add(tx *flow.TransactionBody) bool {
	return t.Cache.Add(tx.ID(), *tx)
}

// ByID returns the transaction with the given ID from the mempool.
func (t *Transactions) ByID(txID flow.Identifier) (*flow.TransactionBody, bool) {
	entity, exists := t.Cache.ByID(txID)
	if !exists {
		return nil, false
	}
	tx, ok := entity.(flow.TransactionBody)
	if !ok {
		panic(fmt.Sprintf("invalid entity in transaction pool (%T)", entity))
	}
	return &tx, true
}

// All returns all transactions from the mempool.
func (t *Transactions) All() []*flow.TransactionBody {
	entities := t.Cache.All()
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
