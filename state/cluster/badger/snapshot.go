package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
)

// Snapshot represents a snapshot of chain state anchored at a particular
// reference block.
type Snapshot struct {
	err     error
	state   *State
	blockID flow.Identifier
}

func (s *Snapshot) Collection() (*flow.Collection, error) {
	if s.err != nil {
		return nil, s.err
	}

	var collection flow.Collection
	err := s.state.db.View(func(tx *badger.Txn) error {

		// get the header for this snapshot
		var header flow.Header
		err := s.head(&header)(tx)
		if err != nil {
			return fmt.Errorf("failed to get snapshot header: %w", err)
		}

		// get the payload
		var payload cluster.Payload
		err = procedure.RetrieveClusterPayload(header.ID(), &payload)(tx)
		if err != nil {
			return fmt.Errorf("failed to get snapshot payload: %w", err)
		}

		// set the collection
		collection = payload.Collection

		return nil
	})

	return &collection, err
}

func (s *Snapshot) Head() (*flow.Header, error) {
	if s.err != nil {
		return nil, s.err
	}

	var head flow.Header
	err := s.state.db.View(func(tx *badger.Txn) error {
		return s.head(&head)(tx)
	})
	return &head, err
}

func (s *Snapshot) Pending() ([]flow.Identifier, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.pending(s.blockID)
}

// head finds the header referenced by the snapshot.
func (s *Snapshot) head(head *flow.Header) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// get the snapshot header
		err := operation.RetrieveHeader(s.blockID, head)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve header for block (%s): %w", s.blockID, err)
		}

		return nil
	}
}

func (s *Snapshot) pending(blockID flow.Identifier) ([]flow.Identifier, error) {

	var pendingIDs []flow.Identifier
	err := s.state.db.View(procedure.LookupBlockChildren(blockID, &pendingIDs))
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
