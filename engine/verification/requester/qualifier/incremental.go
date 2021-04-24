package qualifier

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool"
)

// ChunkDataRequestQualifier always qualifies a request for dispatching. On request getting dispatched, it
// increments the attempt field of request on its underlying mempool.
type ChunkDataRequestQualifier struct {
	pendingRequests mempool.ChunkRequests // used to track requested chunks.
	qualifierFunc   RequestQualifierFunc
}

func NewIncrementalQualifier(requests mempool.ChunkRequests, qualifierFunc RequestQualifierFunc) *ChunkDataRequestQualifier {
	return &ChunkDataRequestQualifier{
		pendingRequests: requests,
		qualifierFunc:   qualifierFunc,
	}
}

// CanDispatchRequest returns true if the chunk request can be dispatched to the network, otherwise
// it returns false.
func (i ChunkDataRequestQualifier) CanDispatchRequest(chunkID flow.Identifier) bool {
	attempts, lastAttempt, retryAfter, exists := i.pendingRequests.RequestInfo(chunkID)
	if !exists {
		return false
	}

	return i.qualifierFunc(attempts, lastAttempt, retryAfter)
}

// OnRequestDispatched encapsulates the bookkeeping logic after dispatching the chunk request
// is done successfully. On request getting dispatched, it
// increments the attempt field of request on its underlying mempool.
func (i *ChunkDataRequestQualifier) OnRequestDispatched(chunkID flow.Identifier) bool {
	return i.pendingRequests.IncrementAttempt(chunkID)
}
