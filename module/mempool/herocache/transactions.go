package herocache

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
	"github.com/onflow/flow-go/module/mempool/stdmap"
)

type Transactions struct {
	c       *stdmap.Backend
	byPayer map[flow.Address]map[flow.Identifier]struct{}
}

// NewTransactions implements a transactions mempool based on hero cache.
func NewTransactions(limit uint32, logger zerolog.Logger, collector module.HeroCacheMetrics) *Transactions {
	byPayer := make(map[flow.Address]map[flow.Identifier]struct{})
	t := &Transactions{
		byPayer: byPayer,
	}

	tracer := &ejectionTracer{transactions: t}
	t.c = stdmap.NewBackend(stdmap.WithBackData(
		herocache.NewCache(limit,
			herocache.DefaultOversizeFactor,
			heropool.LRUEjection,
			logger.With().Str("mempool", "transactions").Logger(),
			collector,
			herocache.WithTracer(tracer))))

	return t
}

// Has checks whether the transaction with the given hash is currently in
// the memory pool.
func (t *Transactions) Has(id flow.Identifier) bool {
	return t.c.Has(id)
}

// Add adds a transaction to the mempool.
func (t *Transactions) Add(tx *flow.TransactionBody) bool {
	added := false
	err := t.c.Run(func(backdata mempool.BackData) error {
		// Warning! reference pointer must be dereferenced before adding to HeroCache.
		// This is crucial for its heap object optimizations.
		txID := tx.ID()
		added = backdata.Add(txID, *tx)
		if !added {
			return nil
		}
		txns, ok := t.byPayer[tx.Payer]
		if !ok {
			txns = make(map[flow.Identifier]struct{})
			t.byPayer[tx.Payer] = txns
		}
		txns[txID] = struct{}{}
		return nil
	})
	if err != nil {
		panic("failed to add transaction to mempool: " + err.Error())
	}
	return added
}

// ByID returns the transaction with the given ID from the mempool.
func (t *Transactions) ByID(txID flow.Identifier) (*flow.TransactionBody, bool) {
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

// All returns all transactions from the mempool. Since it is using the HeroCache, All guarantees returning
// all transactions in the same order as they are added.
func (t *Transactions) All() []*flow.TransactionBody {
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
	err := t.c.Run(func(backdata mempool.BackData) error {
		backdata.Clear()
		t.byPayer = make(map[flow.Address]map[flow.Identifier]struct{})
		return nil
	})
	if err != nil {
		panic("failed to clear transactions mempool: " + err.Error())
	}
}

// Size returns total number of stored transactions.
func (t *Transactions) Size() uint {
	return t.c.Size()
}

// Remove removes transaction from mempool.
func (t *Transactions) Remove(id flow.Identifier) bool {
	removed := false
	err := t.c.Run(func(backdata mempool.BackData) error {
		var entity flow.Entity
		entity, removed = backdata.Remove(id)
		if !removed {
			return nil
		}
		txBody := entity.(flow.TransactionBody)
		t.removeFromIndex(id, txBody.Payer)
		return nil
	})
	if err != nil {
		panic("failed to remove transaction from mempool: " + err.Error())
	}
	return removed
}

// ByPayer retrieves all transactions from the memory pool that are sent
// by the given payer.
func (t *Transactions) ByPayer(payer flow.Address) []*flow.TransactionBody {
	var result []*flow.TransactionBody
	err := t.c.Run(func(backdata mempool.BackData) error {
		ids := t.byPayer[payer]
		for id := range ids {
			entity, exists := backdata.ByID(id)
			if !exists {
				continue
			}
			txBody := entity.(flow.TransactionBody)
			result = append(result, &txBody)
		}
		return nil
	})
	if err != nil {
		panic("failed to get transactions by payer: " + err.Error())
	}
	return result
}

// removeFromIndex removes the transaction with the given ID from the index.
// This function expects that underlying backadata has been locked by the caller, otherwise the operation won't be atomic.
func (t *Transactions) removeFromIndex(id flow.Identifier, payer flow.Address) {
	txns := t.byPayer[payer]
	delete(txns, id)
	if len(txns) == 0 {
		delete(t.byPayer, payer)
	}
}

// ejectionTracer implements herocache.Tracer interface and is used to clean up the index
// when a transaction is ejected from the HeroCache due to capacity or emergency.
type ejectionTracer struct {
	transactions *Transactions
}

var _ herocache.Tracer = (*ejectionTracer)(nil)

func (t *ejectionTracer) EntityEjectionDueToEmergency(ejectedEntity flow.Entity) {
	t.cleanupIndex(ejectedEntity)
}

func (t *ejectionTracer) EntityEjectionDueToFullCapacity(ejectedEntity flow.Entity) {
	t.cleanupIndex(ejectedEntity)
}

// cleanupIndex calls removeFromIndex on the transactions to clean up the index. This is safe since
// backdata is locked by the caller when this function is called.
func (t *ejectionTracer) cleanupIndex(ejectedEntity flow.Entity) {
	txBody := ejectedEntity.(flow.TransactionBody)
	t.transactions.removeFromIndex(txBody.ID(), txBody.Payer)
}
