package badger

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/cluster"
)

type State struct {
	db *badger.DB
}

func NewState(db *badger.DB) (*State, error) {
	state := &State{db: db}
	return state, nil
}

func (s *State) Final() cluster.Snapshot {
	panic("TODO")
}

func (s *State) AtNumber
