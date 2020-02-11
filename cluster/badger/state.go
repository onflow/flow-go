package badger

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
)

type State struct {
	db *badger.DB
}

func NewState(db *badger.DB) (*State, error) {
	state := &State{db: db}
	return state, nil
}

func (s *State) Final() cluster.Snapshot {
	snapshot := &Snapshot{
		final: true,
	}
	return snapshot
}

func (s *State) AtBlockID(blockID flow.Identifier) cluster.Snapshot {
	snapshot := &Snapshot{
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
