package fetcher

import (
	"github.com/onflow/flow-go/engine/access/collection_sync"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/state/protocol"
)

var _ collection_sync.CollectionRequester = (*CollectionRequester)(nil)

// CollectionRequester requests collections from collection nodes on the network.
// It implements the collection_sync.CollectionRequester interface.
type CollectionRequester struct {
	requester module.Requester
	state     protocol.State
}

// NewCollectionRequester creates a new CollectionRequester.
//
// Parameters:
//   - requester: The requester engine for requesting entities from the network
//   - state: Protocol state for finding guarantors
//
// No error returns are expected during normal operation.
func NewCollectionRequester(
	requester module.Requester,
	state protocol.State,
) *CollectionRequester {
	return &CollectionRequester{
		requester: requester,
		state:     state,
	}
}

// RequestCollectionsByGuarantees requests collections by their guarantees from collection nodes on the network.
// For each guarantee, it finds the guarantors and requests the collection from them.
//
// No error returns are expected during normal operation.
func (cr *CollectionRequester) RequestCollectionsByGuarantees(guarantees []*flow.CollectionGuarantee) error {
	for _, guarantee := range guarantees {
		// Find guarantors for this guarantee
		guarantors, err := protocol.FindGuarantors(cr.state, guarantee)
		if err != nil {
			// Failed to find guarantors - this could happen if the reference block is unknown
			// or if the cluster is not found. Skip this collection rather than failing entirely.
			continue
		}

		// Request the collection from the guarantors
		cr.requester.EntityByID(guarantee.CollectionID, filter.HasNodeID[flow.Identity](guarantors...))
	}

	// Force immediate dispatch of all pending requests
	cr.requester.Force()

	return nil
}
