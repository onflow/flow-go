package collections

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/engine/access/ingestion2"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

var _ ingestion2.CollectionRequester = (*CollectionRequester)(nil)

// CollectionRequester requests collections from collection nodes on the network.
// It implements the ingestion2.CollectionRequester interface.
type CollectionRequester struct {
	requester  module.Requester
	state      protocol.State
	guarantees storage.Guarantees
}

// NewCollectionRequester creates a new CollectionRequester.
//
// Parameters:
//   - requester: The requester engine for requesting entities from the network
//   - state: Protocol state for finding guarantors
//   - guarantees: Guarantees storage for retrieving collection guarantees by collection ID
//
// No error returns are expected during normal operation.
func NewCollectionRequester(
	requester module.Requester,
	state protocol.State,
	guarantees storage.Guarantees,
) *CollectionRequester {
	return &CollectionRequester{
		requester:  requester,
		state:      state,
		guarantees: guarantees,
	}
}

// RequestCollections requests collections by their IDs from collection nodes on the network.
// For each collection, it finds the guarantors and requests the collection from them.
//
// No error returns are expected during normal operation.
func (cr *CollectionRequester) RequestCollections(ids []flow.Identifier) error {
	for _, collectionID := range ids {
		// Retrieve the guarantee for this collection
		guarantee, err := cr.guarantees.ByCollectionID(collectionID)
		if err != nil {
			// If guarantee is not found, we can't determine guarantors, so skip this collection
			// This can happen if the collection hasn't been finalized yet or if it's from a fork
			if errors.Is(err, storage.ErrNotFound) {
				continue
			}
			return fmt.Errorf("failed to retrieve guarantee for collection %v: %w", collectionID, err)
		}

		// Find guarantors for this guarantee
		guarantors, err := protocol.FindGuarantors(cr.state, guarantee)
		if err != nil {
			// Failed to find guarantors - this could happen if the reference block is unknown
			// or if the cluster is not found. Skip this collection rather than failing entirely.
			continue
		}

		// Request the collection from the guarantors
		cr.requester.EntityByID(collectionID, filter.HasNodeID[flow.Identity](guarantors...))
	}

	// Force immediate dispatch of all pending requests
	cr.requester.Force()

	return nil
}

