// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
)

type State struct {
	metrics  module.ComplianceMetrics
	db       *badger.DB
	headers  storage.Headers
	seals    storage.Seals
	index    storage.Index
	payloads storage.Payloads
	blocks   storage.Blocks
	epoch    struct {
		setups   storage.EpochSetups
		commits  storage.EpochCommits
		statuses storage.EpochStatuses
	}
	consumer protocol.Consumer
	cfg      Config
}

// NewState initializes a new state backed by a badger database, applying the
// optional configuration parameters.
func NewState(
	metrics module.ComplianceMetrics,
	db *badger.DB,
	headers storage.Headers,
	seals storage.Seals,
	index storage.Index,
	payloads storage.Payloads,
	blocks storage.Blocks,
	setups storage.EpochSetups,
	commits storage.EpochCommits,
	statuses storage.EpochStatuses,
	consumer protocol.Consumer,
) (*State, error) {

	s := &State{
		metrics:  metrics,
		db:       db,
		headers:  headers,
		seals:    seals,
		index:    index,
		payloads: payloads,
		blocks:   blocks,
		epoch: struct {
			setups   storage.EpochSetups
			commits  storage.EpochCommits
			statuses storage.EpochStatuses
		}{
			setups:   setups,
			commits:  commits,
			statuses: statuses,
		},
		consumer: consumer,
		cfg:      DefaultConfig(),
	}

	return s, nil
}

func (s *State) Params() protocol.Params {
	return &Params{state: s}
}

func (s *State) Sealed() protocol.Snapshot {
	// retrieve the latest sealed height
	var sealed uint64
	err := s.db.View(operation.RetrieveSealedHeight(&sealed))
	if err != nil {
		return NewInvalidSnapshot(fmt.Errorf("could not retrieve sealed height: %w", err))
	}
	return s.AtHeight(sealed)
}

func (s *State) Final() protocol.Snapshot {
	// retrieve the latest finalized height
	var finalized uint64
	err := s.db.View(operation.RetrieveFinalizedHeight(&finalized))
	if err != nil {
		return NewInvalidSnapshot(fmt.Errorf("could not retrieve finalized height: %w", err))
	}
	return s.AtHeight(finalized)
}

func (s *State) AtHeight(height uint64) protocol.Snapshot {
	// retrieve the block ID for the finalized height
	var blockID flow.Identifier
	err := s.db.View(operation.LookupBlockHeight(height, &blockID))
	if err != nil {
		return NewInvalidSnapshot(fmt.Errorf("could not look up block by height: %w", err))
	}
	return s.AtBlockID(blockID)
}

func (s *State) AtBlockID(blockID flow.Identifier) protocol.Snapshot {
	return NewSnapshot(s, blockID)
}

func (s *State) Mutate() protocol.Mutator {
	m := &Mutator{
		state: s,
	}
	return m
}
