package mempool

import (
	"github.com/onflow/flow-go/model/flow"
)

// Transactions represents a concurrency-safe memory pool for transactions.
type Transactions interface {
	Mempool[flow.Identifier, *flow.TransactionBody]
	// ByPayer retrieves all transactions from the memory pool that are sent
	// by the given payer.
	ByPayer(payer flow.Address) []*flow.TransactionBody
}
