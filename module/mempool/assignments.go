package mempool

import (
	chunkmodels "github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
)

// Assignments represents a concurrency-safe memory pool for chunk assignments.
type Assignments Mempool[flow.Identifier, *chunkmodels.Assignment]
