package requester

import (
	"github.com/onflow/flow-go/model/verification"
)

// ChunkDataRequestQualifier encapsulates the logic of deciding whether to dispatch a chunk data request
// or to hold on it.
// It also encapsulates the bookkeeping logic of dispatched chunk data requests.
type ChunkDataRequestQualifier interface {
	// CanDispatchRequest returns true if the chunk request can be dispatched to the network, otherwise
	// it returns false.
	CanDispatchRequest(request verification.ChunkRequestStatus) bool

	// OnRequestDispatched encapsulates the bookkeeping logic after dispatching the chunk request
	// is done successfully.
	OnRequestDispatched(request *verification.ChunkDataPackRequest) bool
}
