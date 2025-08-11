package mempool

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
)

// ChunkStatuses is an in-memory storage for maintaining the chunk status data objects.
type ChunkStatuses Mempool[flow.Identifier, *verification.ChunkStatus]
