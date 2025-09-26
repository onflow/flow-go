package badger

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	clusterState "github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/procedure"
)

// Snapshot represents a snapshot of chain state anchored at a particular
// reference block.
type Snapshot struct {
	err     error
	state   *State
	blockID flow.Identifier
}

var _ clusterState.Snapshot = (*Snapshot)(nil)

func (s *Snapshot) Collection() (*flow.Collection, error) {
	if s.err != nil {
		return nil, s.err
	}

	// get the payload
	var payload cluster.Payload
	err := procedure.RetrieveClusterPayload(s.state.db.Reader(), s.blockID, &payload)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot payload: %w", err)
	}

	collection := payload.Collection
	return &collection, nil
}

func (s *Snapshot) Head() (*flow.Header, error) {
	if s.err != nil {
		return nil, s.err
	}

	var head flow.Header
	err := s.head(&head)
	return &head, err
}

// Pending returns all pending block IDs that descend from the snapshot's reference block.
// Note, the caller must have checked that the block of the snapshot does exist in the database.
// This is currently true, because the Snapshot instance is only created by AtBlockID and AtHeight
// method of State, which both check the existence of the block first.
func (s *Snapshot) Pending() ([]flow.Identifier, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.pending(s.blockID)
}

// head finds the header referenced by the snapshot.
func (s *Snapshot) head(head *flow.Header) error {
	// get the snapshot header
	err := operation.RetrieveHeader(s.state.db.Reader(), s.blockID, head)
	if err != nil {
		return fmt.Errorf("could not retrieve header for block (%s): %w", s.blockID, err)
	}
	return nil
}

func (s *Snapshot) pending(blockID flow.Identifier) ([]flow.Identifier, error) {
	var pendingIDs flow.IdentifierList
	err := operation.RetrieveBlockChildren(s.state.db.Reader(), blockID, &pendingIDs)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return nil, fmt.Errorf("could not get pending block %v: %w", blockID, err)
		}

		// err not found means two case:
		// 1. the block doesn't exist
		// 2. the block exists but has no children
		// since the snapshot is created only when s.err == nil, which means the block exists,
		// so only case 2 is possible here. In this case, we just return
		// empty children list.
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
