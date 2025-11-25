package herocache

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
	"github.com/onflow/flow-go/module/mempool/stdmap"
)

type Transactions struct {
	*stdmap.Backend[flow.Identifier, *flow.TransactionBody]
	byPayer map[flow.Address]map[flow.Identifier]struct{}
}

var _ mempool.Transactions = (*Transactions)(nil)

// NewTransactions implements a transactions mempool based on hero cache.
func NewTransactions(limit uint32, logger zerolog.Logger, collector module.HeroCacheMetrics) *Transactions {
	byPayer := make(map[flow.Address]map[flow.Identifier]struct{})
	t := &Transactions{
		byPayer: byPayer,
	}

	tracer := &ejectionTracer{transactions: t}
	t.Backend = stdmap.NewBackend(
		stdmap.WithMutableBackData[flow.Identifier, *flow.TransactionBody](
			herocache.NewCache[*flow.TransactionBody](limit,
				herocache.DefaultOversizeFactor,
				heropool.LRUEjection,
				logger.With().Str("mempool", "transactions").Logger(),
				collector,
				herocache.WithTracer[*flow.TransactionBody](tracer))))

	return t
}

// Add adds a transaction to the mempool.
func (t *Transactions) Add(txID flow.Identifier, tx *flow.TransactionBody) bool {
	added := false
	err := t.Run(func(backdata mempool.BackData[flow.Identifier, *flow.TransactionBody]) error {
		// Warning! reference pointer must be dereferenced before adding to HeroCache.
		// This is crucial for its heap object optimizations.
		added = backdata.Add(txID, tx)
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

// Clear removes all transactions stored in this mempool.
func (t *Transactions) Clear() {
	err := t.Run(func(backdata mempool.BackData[flow.Identifier, *flow.TransactionBody]) error {
		backdata.Clear()
		t.byPayer = make(map[flow.Address]map[flow.Identifier]struct{})
		return nil
	})
	if err != nil {
		panic("failed to clear transactions mempool: " + err.Error())
	}
}

// Remove removes transaction from mempool.
func (t *Transactions) Remove(id flow.Identifier) bool {
	removed := false
	err := t.Run(func(backdata mempool.BackData[flow.Identifier, *flow.TransactionBody]) error {
		var txBody *flow.TransactionBody
		txBody, removed = backdata.Remove(id)
		if !removed {
			return nil
		}
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
	err := t.Run(func(backdata mempool.BackData[flow.Identifier, *flow.TransactionBody]) error {
		ids := t.byPayer[payer]
		for id := range ids {
			txBody, exists := backdata.Get(id)
			if !exists {
				continue
			}
			result = append(result, txBody)
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

var _ herocache.Tracer[*flow.TransactionBody] = (*ejectionTracer)(nil)

// EntityEjectionDueToEmergency calls removeFromIndex on the transactions to clean up the index. This is safe since
// backdata is locked by the caller when this function is called.
func (t *ejectionTracer) EntityEjectionDueToEmergency(txBody *flow.TransactionBody) {
	t.transactions.removeFromIndex(txBody.ID(), txBody.Payer)
}

// EntityEjectionDueToFullCapacity calls removeFromIndex on the transactions to clean up the index. This is safe since
// backdata is locked by the caller when this function is called.
func (t *ejectionTracer) EntityEjectionDueToFullCapacity(txBody *flow.TransactionBody) {
	t.transactions.removeFromIndex(txBody.ID(), txBody.Payer)
}
