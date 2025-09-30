package badger

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	clusterState "github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

// Snapshot pertains to a specific fork of the collector cluster consensus. Specifically,
// it references one block denoted as the `Head`. This Snapshot type is for collector
// clusters, so we are referencing a cluster block, aka collection, here.
//
// This implementation must be used for KNOWN reference BLOCKs only.
type Snapshot struct {
	state   *State
	blockID flow.Identifier
}

var _ clusterState.Snapshot = (*Snapshot)(nil)

// newSnapshot instantiates a new snapshot for the given collection ID.
// CAUTION: This constructor must be called for KNOWN blocks.
// For unknown blocks, please use `invalid.NewSnapshot` or `invalid.NewSnapshotf`.
func newSnapshot(state *State, blockID flow.Identifier) *Snapshot {
	return &Snapshot{
		state:   state,
		blockID: blockID,
	}
}

// Collection returns the collection designated as the reference for this
// snapshot. Technically, this is a portion of the payload of a cluster block.
//
// By contract of the constructor, the blockID must correspond to a known collection in the database.
// No error returns are expected during normal operation.
func (s *Snapshot) Collection() (*flow.Collection, error) {
	// get the payload
	var payload cluster.Payload
	err := operation.RetrieveClusterPayload(s.state.db.Reader(), s.blockID, &payload)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot payload: %w", err)
	}

	collection := payload.Collection
	return &collection, nil
}

// Head returns the header of the collection that designated as the reference for this
// snapshot.
//
// By contract of the constructor, the blockID must correspond to a known collection in the database.
// No error returns are expected during normal operation.
func (s *Snapshot) Head() (*flow.Header, error) {
	var head flow.Header
	err := operation.RetrieveHeader(s.state.db.Reader(), s.blockID, &head)
	if err != nil {
		// `storage.ErrNotFound` is the only error that the storage layer may return other than exceptions.
		// In the context of this call, `s.blockID` should correspond to a known block, so receiving a
		// `storage.ErrNotFound` is an exception here.
		return nil, irrecoverable.NewExceptionf("could not retrieve header for block (%s): %w", s.blockID, err)
	}
	return &head, err
}

// Pending returns the IDs of all collections descending from the snapshot's head collection.
// The result is ordered such that parents are included before their children. While only valid
// descendants will be returned, note that the descendants may not be finalized yet.
// By contract of the constructor, the blockID must correspond to a known collection in the database.
// No error returns are expected during normal operation.
func (s *Snapshot) Pending() ([]flow.Identifier, error) {
	return s.pending(s.blockID)
}

// pending returns a slice with all blocks descending from the given blockID (children, grandchildren, etc).
// CAUTION: this function behaves only correctly for known blocks, which should always be the case as
// required by the constructor.
// No error returns are expected during normal operation.
func (s *Snapshot) pending(blockID flow.Identifier) ([]flow.Identifier, error) {
	var pendingIDs flow.IdentifierList
	err := operation.RetrieveBlockChildren(s.state.db.Reader(), blockID, &pendingIDs)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return nil, fmt.Errorf("could not get pending block %v: %w", blockID, err)
		}

		// The low-level storage returns `storage.ErrNotFound` in two cases:
		// 1. the block/collection is unknown
		// 2. the block/collection is known but no children have been indexed yet
		// By contract of the constructor, the blockID must correspond to a known collection in the database.
		// A snapshot with s.err == nil is only created for known blocks. Hence, only case 2 is
		// possible here, and we just return an empty list.
	}

	for _, pendingID := range pendingIDs {
		additionalIDs, err := s.pending(pendingID)
		if err != nil {
			return nil, fmt.Errorf("could not get pending grandchildren: %w", err)
		}
		pendingIDs = append(pendingIDs, additionalIDs...)
	}
	return pendingIDs, nil
}
