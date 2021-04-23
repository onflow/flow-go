package qualifier

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool"
)

// IncrementalQualifier always qualifies a request for dispatching. On request getting dispatched, it
// increments the attempt field of request on its underlying mempool.
type IncrementalQualifier struct {
	pendingRequests mempool.ChunkRequests // used to track requested chunks.
}

func NewIncrementalQualifier(requests mempool.ChunkRequests) *IncrementalQualifier {
	return &IncrementalQualifier{pendingRequests: requests}
}

// CanDispatchRequest always returns true. Its sole purpose is to satisfy the interface implementation.
func (i *IncrementalQualifier) CanDispatchRequest(chunkID flow.Identifier) bool {
	return true
}

// OnRequestDispatched encapsulates the bookkeeping logic after dispatching the chunk request
// is done successfully. On request getting dispatched, it
// increments the attempt field of request on its underlying mempool.
func (i *IncrementalQualifier) OnRequestDispatched(chunkID flow.Identifier) bool {
	return i.pendingRequests.IncrementAttempt(chunkID)
}
