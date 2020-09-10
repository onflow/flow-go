// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
	cache "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

type State struct {
	metrics     module.ComplianceMetrics
	db          *badger.DB
	headers     storage.Headers
	seals       storage.Seals
	index       storage.Index
	payloads    storage.Payloads
	blocks      storage.Blocks
	setups      storage.EpochSetups
	commits     storage.EpochCommits
	epochStates storage.EpochStates
	cfg         Config
}

// NewState initializes a new state backed by a badger database, applying the
// optional configuration parameters.
func NewState(
	metrics module.ComplianceMetrics, cacheMetrics module.CacheMetrics, db *badger.DB,
	headers storage.Headers, seals storage.Seals, index storage.Index, payloads storage.Payloads, blocks storage.Blocks,
	setups storage.EpochSetups, commits storage.EpochCommits,
) (*State, error) {

	s := &State{
		metrics:     metrics,
		db:          db,
		headers:     headers,
		seals:       seals,
		index:       index,
		payloads:    payloads,
		blocks:      blocks,
		setups:      setups,
		commits:     commits,
		epochStates: cache.NewEpochStates(cacheMetrics, db), // TODO (?) we might have to inject this in scaffold to avoid circular dependency
		cfg:         DefaultConfig(),
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
		return &BlockSnapshot{err: fmt.Errorf("could not retrieve sealed height: %w", err)}
	}

	return s.AtHeight(sealed)
}

func (s *State) Final() protocol.Snapshot {

	// retrieve the latest finalized height
	var finalized uint64
	err := s.db.View(operation.RetrieveFinalizedHeight(&finalized))
	if err != nil {
		return &BlockSnapshot{err: fmt.Errorf("could not retrieve finalized height: %w", err)}
	}

	return s.AtHeight(finalized)
}

func (s *State) AtHeight(height uint64) protocol.Snapshot {

	// retrieve the block ID for the finalized height
	var blockID flow.Identifier
	err := s.db.View(operation.LookupBlockHeight(height, &blockID))
	if err != nil {
		return &BlockSnapshot{err: fmt.Errorf("could not look up block by height: %w", err)}
	}

	return s.AtBlockID(blockID)
}

func (s *State) AtBlockID(blockID flow.Identifier) protocol.Snapshot {
	snapshot := &BlockSnapshot{
		state:   s,
		blockID: blockID,
	}
	return snapshot
}

func (s *State) AtEpoch(counter uint64) protocol.EpochSnapshot {
	snapshot := &EpochSnapshot{
		state:   s,
		counter: counter,
	}
	return snapshot
}

func (s *State) Mutate() protocol.Mutator {
	m := &Mutator{
		state: s,
	}
	return m
}
