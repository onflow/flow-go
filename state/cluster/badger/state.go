package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/state/cluster"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

type State struct {
	db      *badger.DB
	chainID string // aka cluster ID
}

func NewState(db *badger.DB, chainID string) (*State, error) {
	state := &State{
		db:      db,
		chainID: chainID,
	}
	return state, nil
}

func (s *State) Final() cluster.Snapshot {

	// get the finalized block ID
	var blockID flow.Identifier
	err := s.db.View(func(tx *badger.Txn) error {
		var boundary uint64
		err := operation.RetrieveClusterFinalizedHeight(s.chainID, &boundary)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve finalized boundary: %w", err)
		}

		err = operation.LookupClusterBlockHeight(s.chainID, boundary, &blockID)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve finalized ID: %w", err)
		}

		return nil
	})
	if err != nil {
		return &Snapshot{
			err: err,
		}
	}

	snapshot := &Snapshot{
		state:   s,
		blockID: blockID,
	}
	return snapshot
}

func (s *State) AtBlockID(blockID flow.Identifier) cluster.Snapshot {
	snapshot := &Snapshot{
		state:   s,
		blockID: blockID,
	}
	return snapshot
}

func (s *State) Mutate() cluster.Mutator {
	mutator := &Mutator{
		state: s,
	}
	return mutator
}
