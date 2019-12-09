// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"math"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/protocol"
)

type State struct {
	db *badger.DB
}

func NewState(db *badger.DB) (*State, error) {
	s := &State{
		db: db,
	}
	return s, nil
}

func (s *State) Final() protocol.Snapshot {
	sn := &Snapshot{
		state:  s,
		number: math.MaxUint64,
		hash:   nil,
	}
	return sn
}

func (s *State) AtNumber(number uint64) protocol.Snapshot {
	sn := &Snapshot{
		state:  s,
		number: number,
		hash:   nil,
	}
	return sn
}

func (s *State) AtHash(hash crypto.Hash) protocol.Snapshot {
	sn := &Snapshot{
		state:  s,
		number: 0,
		hash:   hash,
	}
	return sn
}

func (s *State) Mutate() protocol.Mutator {
	m := &Mutator{
		state: s,
	}
	return m
}
