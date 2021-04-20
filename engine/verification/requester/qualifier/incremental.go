package qualifier

import (
	"github.com/onflow/flow-go/model/verification"
	"github.com/onflow/flow-go/module/mempool"
)

type IncrementalQualifier struct {
	pendingRequests mempool.ChunkRequests // used to track requested chunks.
}

func NewIncrementalQualifier(requests mempool.ChunkRequests) *IncrementalQualifier {
	return &IncrementalQualifier{pendingRequests: requests}
}

// CanDispatchRequest returns true if the chunk request can be dispatched to the network, otherwise
// it returns false.
func (i *IncrementalQualifier) CanDispatchRequest(request verification.ChunkRequestStatus) bool {
	return true
}

// OnRequestDispatched encapsulates the bookkeeping logic after dispatching the chunk request
// is done successfully.
func (i *IncrementalQualifier) OnRequestDispatched(request *verification.ChunkRequestStatus) bool {
	return i.pendingRequests.IncrementAttempt(request.ID())
}
