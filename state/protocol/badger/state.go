// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
)

type State struct {
	metrics module.ComplianceMetrics
	db      *badger.DB
	headers storage.Headers
	blocks  storage.Blocks
	seals   storage.Seals
	epoch   struct {
		setups   storage.EpochSetups
		commits  storage.EpochCommits
		statuses storage.EpochStatuses
	}
}

func Bootstrap(
	metrics module.ComplianceMetrics,
	db *badger.DB,
	headers storage.Headers,
	seals storage.Seals,
	blocks storage.Blocks,
	setups storage.EpochSetups,
	commits storage.EpochCommits,
	statuses storage.EpochStatuses,
	stateRoot *StateRoot,
) (*State, error) {
	isBootstrapped, err := IsBootstrapped(db)
	if err != nil {
		return nil, fmt.Errorf("failed to determine whether database contains bootstrapped state: %w", err)
	}
	if isBootstrapped {
		return nil, fmt.Errorf("expected empty database")
	}
	state := newState(metrics, db, headers, seals, blocks, setups, commits, statuses)

	err = operation.RetryOnConflict(db.Update, func(tx *badger.Txn) error {
		// 1) insert the root block with its payload into the state and index it
		err = state.blocks.StoreTx(stateRoot.Block())(tx)
		if err != nil {
			return fmt.Errorf("could not insert root block: %w", err)
		}
		err = operation.InsertBlockValidity(stateRoot.Block().ID(), true)(tx)
		if err != nil {
			return fmt.Errorf("could not mark root block as valid: %w", err)
		}

		err = operation.IndexBlockHeight(stateRoot.Block().Header.Height, stateRoot.Block().ID())(tx)
		if err != nil {
			return fmt.Errorf("could not index root block: %w", err)
		}
		// root block has no parent, so only needs to add one index
		// to indicate the root block has no child yet
		err = operation.InsertBlockChildren(stateRoot.Block().ID(), nil)(tx)
		if err != nil {
			return fmt.Errorf("could not initialize root child index: %w", err)
		}

		// 2) insert the root execution result into the database and index it
		err = operation.InsertExecutionResult(stateRoot.Result())(tx)
		if err != nil {
			return fmt.Errorf("could not insert root result: %w", err)
		}
		err = operation.IndexExecutionResult(stateRoot.Block().ID(), stateRoot.Result().ID())(tx)
		if err != nil {
			return fmt.Errorf("could not index root result: %w", err)
		}

		// 3) insert the root block seal into the database and index it
		err = operation.InsertSeal(stateRoot.Seal().ID(), stateRoot.Seal())(tx)
		if err != nil {
			return fmt.Errorf("could not insert root seal: %w", err)
		}
		err = operation.IndexBlockSeal(stateRoot.Block().ID(), stateRoot.Seal().ID())(tx)
		if err != nil {
			return fmt.Errorf("could not index root block seal: %w", err)
		}

		// 4) initialize the current protocol state values
		err = operation.InsertStartedView(stateRoot.Block().Header.ChainID, stateRoot.Block().Header.View)(tx)
		if err != nil {
			return fmt.Errorf("could not insert started view: %w", err)
		}
		err = operation.InsertVotedView(stateRoot.Block().Header.ChainID, stateRoot.Block().Header.View)(tx)
		if err != nil {
			return fmt.Errorf("could not insert started view: %w", err)
		}
		err = operation.InsertRootHeight(stateRoot.Block().Header.Height)(tx)
		if err != nil {
			return fmt.Errorf("could not insert root height: %w", err)
		}
		err = operation.InsertFinalizedHeight(stateRoot.Block().Header.Height)(tx)
		if err != nil {
			return fmt.Errorf("could not insert finalized height: %w", err)
		}
		err = operation.InsertSealedHeight(stateRoot.Block().Header.Height)(tx)
		if err != nil {
			return fmt.Errorf("could not insert sealed height: %w", err)
		}

		// 5) initialize values related to the epoch logic
		err = state.epoch.setups.StoreTx(stateRoot.EpochSetupEvent())(tx)
		if err != nil {
			return fmt.Errorf("could not insert EpochSetup event: %w", err)
		}
		err = state.epoch.commits.StoreTx(stateRoot.EpochCommitEvent())(tx)
		if err != nil {
			return fmt.Errorf("could not insert EpochCommit event: %w", err)
		}
		status, err := flow.NewEpochStatus(stateRoot.Block().ID(), stateRoot.EpochSetupEvent().ID(), stateRoot.EpochCommitEvent().ID(), flow.ZeroID, flow.ZeroID)
		if err != nil {
			return fmt.Errorf("could not construct root epoch status: %w", err)
		}
		err = state.epoch.statuses.StoreTx(stateRoot.Block().ID(), status)(tx)
		if err != nil {
			return fmt.Errorf("could not insert EpochStatus: %w", err)
		}

		state.metrics.FinalizedHeight(stateRoot.Block().Header.Height)
		state.metrics.BlockFinalized(stateRoot.Block())

		state.metrics.SealedHeight(stateRoot.Block().Header.Height)
		state.metrics.BlockSealed(stateRoot.Block())

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("bootstrapping failed: %w", err)
	}

	return state, nil
}

func OpenState(
	metrics module.ComplianceMetrics,
	db *badger.DB,
	headers storage.Headers,
	seals storage.Seals,
	blocks storage.Blocks,
	setups storage.EpochSetups,
	commits storage.EpochCommits,
	statuses storage.EpochStatuses,
) (*State, *StateRoot, error) {
	isBootstrapped, err := IsBootstrapped(db)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to determine whether database contains bootstrapped state: %w", err)
	}
	if !isBootstrapped {
		return nil, nil, fmt.Errorf("expected database to contain bootstrapped state")
	}
	state := newState(metrics, db, headers, seals, blocks, setups, commits, statuses)

	// read root block from database:
	var rootHeight uint64
	err = db.View(operation.RetrieveRootHeight(&rootHeight))
	if err != nil {
		return nil, nil, fmt.Errorf("failed retrieve root height: %w", err)
	}
	rootBlock, err := blocks.ByHeight(rootHeight)
	if err != nil {
		return nil, nil, fmt.Errorf("failed retrieve root block: %w", err)
	}

	// read root execution result
	var resultID flow.Identifier
	err = db.View(operation.LookupExecutionResult(rootBlock.ID(), &resultID))
	if err != nil {
		return nil, nil, fmt.Errorf("failed retrieve root block's execution result ID: %w", err)
	}
	var result flow.ExecutionResult
	err = db.View(operation.RetrieveExecutionResult(resultID, &result))
	if err != nil {
		return nil, nil, fmt.Errorf("failed retrieve root block's execution result: %w", err)
	}

	// read root seal
	seal, err := seals.ByBlockID(rootBlock.ID())
	if err != nil {
		return nil, nil, fmt.Errorf("failed retrieve root block's seal: %w", err)
	}

	// read root seal
	epochStatus, err := statuses.ByBlockID(rootBlock.ID())
	if err != nil {
		return nil, nil, fmt.Errorf("failed retrieve root block's epoch status: %w", err)
	}
	epochSetup, err := setups.ByID(epochStatus.CurrentEpoch.SetupID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed retrieve root epochs's setup event: %w", err)
	}

	// construct state Root
	stateRoot, err := NewStateRoot(rootBlock, &result, seal, epochSetup.FirstView)
	if err != nil {
		return nil, nil, fmt.Errorf("constructing state root failed: %w", err)
	}

	return state, stateRoot, nil
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

// newState initializes a new state backed by the provided a badger database,
// mempools and service components.
// The parameter `expectedBootstrappedState` indicates whether or not the database
// is expected to contain a an already bootstrapped state or not
func newState(
	metrics module.ComplianceMetrics,
	db *badger.DB,
	headers storage.Headers,
	seals storage.Seals,
	blocks storage.Blocks,
	setups storage.EpochSetups,
	commits storage.EpochCommits,
	statuses storage.EpochStatuses,
) *State {
	return &State{
		metrics: metrics,
		db:      db,
		headers: headers,
		seals:   seals,
		blocks:  blocks,
		epoch: struct {
			setups   storage.EpochSetups
			commits  storage.EpochCommits
			statuses storage.EpochStatuses
		}{
			setups:   setups,
			commits:  commits,
			statuses: statuses,
		},
	}
}

// IsBootstrapped returns whether or not the database contains a bootstrapped state
func IsBootstrapped(db *badger.DB) (bool, error) {
	var finalized uint64
	err := db.View(operation.RetrieveFinalizedHeight(&finalized))
	if errors.Is(err, storage.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("retrieving finalized height failed: %w", err)
	}
	return true, nil
}
