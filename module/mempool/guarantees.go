package mempool

import (
	"github.com/onflow/flow-go/model/flow"
)

// Guarantees represents a concurrency-safe memory pool for collection guarantees.
type Guarantees Mempool[flow.Identifier, *flow.CollectionGuarantee]
