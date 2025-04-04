package mempool

import (
	"github.com/onflow/flow-go/model/flow"
)

// TransactionTimings represents a concurrency-safe memory pool for transaction timings.
type TransactionTimings Mempool[flow.Identifier, *flow.TransactionTiming]
