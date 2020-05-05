// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"fmt"
	"math"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
)

type State struct {
	db               *badger.DB
	clusters         uint
	validationBlocks uint64

	// lazily set the chain ID once for each instance, we use this to ensure
	// all blocks returned by protocol state are part of the main chain
	chainID string
}

// NewState initializes a new state backed by a badger database, applying the
// optional configuration parameters.
func NewState(db *badger.DB, options ...func(*State)) (*State, error) {
	s := &State{
		db:               db,
		clusters:         1,
		validationBlocks: 64,
	}
	for _, option := range options {
		option(s)
	}

	if s.clusters < 1 {
		return nil, fmt.Errorf("must have clusters>0 (actual=%d)", s.clusters)
	}

	return s, nil
}

func (s *State) Final() protocol.Snapshot {
	sn := &Snapshot{
		state:   s,
		number:  math.MaxUint64,
		blockID: flow.ZeroID,
	}
	return sn
}

func (s *State) AtNumber(number uint64) protocol.Snapshot {
	sn := &Snapshot{
		state:   s,
		number:  number,
		blockID: flow.ZeroID,
	}
	return sn
}

func (s *State) AtBlockID(blockID flow.Identifier) protocol.Snapshot {
	sn := &Snapshot{
		state:   s,
		number:  0,
		blockID: blockID,
	}
	return sn
}

func (s *State) Mutate() protocol.Mutator {
	m := &Mutator{
		state: s,
	}
	return m
}

// getChainID retrieves the chain ID of the main chain, ensuring that
// the ID is only actually retrieved once.
func (s *State) getChainID(chainID *string) func(tx *badger.Txn) error {
	return func(tx *badger.Txn) error {

		// if the chain ID is already set, return it
		if s.chainID != "" {
			*chainID = s.chainID
			return nil
		}

		// otherwise, retrieve the finalized head to get the chain ID
		// we could get any block here, since they all use the same chain ID
		var final flow.Header
		err := procedure.RetrieveLatestFinalizedHeader(&final)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve finalized header to check chain ID: %w", err)
		}

		// only if we have successfully retrieved the header, set the chain ID
		s.chainID = final.ChainID
		*chainID = s.chainID

		return nil
	}
}
