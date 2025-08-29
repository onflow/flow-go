package badger

import (
	"fmt"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	clusterState "github.com/onflow/flow-go/state/cluster"
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

	var collection flow.Collection

	// get the payload
	var payload cluster.Payload
	err := procedure.RetrieveClusterPayload(s.state.db.Reader(), s.blockID, &payload)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot payload: %w", err)
	}

	// set the collection
	collection = payload.Collection

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
	err := procedure.LookupBlockChildren(s.state.db.Reader(), blockID, &pendingIDs)
	if err != nil {
		return nil, fmt.Errorf("could not get pending children: %w", err)
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
