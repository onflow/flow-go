package badger

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
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
	snapshot := &Snapshot{
		state: s,
		final: true,
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
