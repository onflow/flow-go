package requester

import (
	"github.com/onflow/flow-go/model/flow"
)

// ChunkDataRequestQualifier encapsulates the logic of deciding whether to dispatch a chunk data request
// or to hold on it.
// It also encapsulates the bookkeeping logic of dispatched chunk data requests.
type ChunkDataRequestQualifier interface {
	// CanDispatchRequest returns true if the chunk request can be dispatched to the network, otherwise
	// it returns false.
	CanDispatchRequest(chunkID flow.Identifier) bool

	// OnRequestDispatched encapsulates the bookkeeping logic after dispatching the chunk request
	// is done successfully.
	OnRequestDispatched(chunkID flow.Identifier) bool
}
