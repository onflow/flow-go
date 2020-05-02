// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

type State struct {
	db         *badger.DB
	clusters   uint
	identities storage.Identities
	headers    storage.Headers
	payloads   storage.Payloads
	seals      storage.Seals
	commits    storage.Commits
}

// NewState initializes a new state backed by a badger database, applying the
// optional configuration parameters.
func NewState(db *badger.DB, identities storage.Identities, headers storage.Headers, payloads storage.Payloads, seals storage.Seals, commits storage.Commits, options ...func(*State)) (*State, error) {
	s := &State{
		db:         db,
		clusters:   1,
		identities: identities,
		headers:    headers,
		payloads:   payloads,
		seals:      seals,
		commits:    commits,
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

	// retrieve the latest finalized block at this height
	var finalized uint64
	err := s.db.View(operation.RetrieveFinalizedHeight(&finalized))
	if err != nil {
		return &Snapshot{err: err}
	}

	return s.AtHeight(finalized)
}

func (s *State) Executed() protocol.Snapshot {

	// retrieve the latest finalized block at this height
	var executed uint64
	err := s.db.View(operation.RetrieveExecutedHeight(&executed))
	if err != nil {
		return &Snapshot{err: err}
	}

	return s.AtHeight(executed)
}

func (s *State) Sealed() protocol.Snapshot {

	// retrieve the latest finalized block at this height
	var sealed uint64
	err := s.db.View(operation.RetrieveSealedHeight(&sealed))
	if err != nil {
		return &Snapshot{err: err}
	}

	return s.AtHeight(sealed)
}

func (s *State) AtHeight(height uint64) protocol.Snapshot {

	// retrieve the block ID for the finalized height
	var blockID flow.Identifier
	err := s.db.View(operation.LookupBlockHeight(height, &blockID))
	if err != nil {
		return &Snapshot{err: err}
	}

	snapshot := &Snapshot{
		state:   s,
		blockID: blockID,
	}

	return snapshot
}

func (s *State) AtBlockID(blockID flow.Identifier) protocol.Snapshot {
	snapshot := &Snapshot{
		state:   s,
		blockID: blockID,
	}
	return snapshot
}

func (s *State) Mutate() protocol.Mutator {
	m := &Mutator{
		state: s,
	}
	return m
}
