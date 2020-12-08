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
	tracer   module.Tracer
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

type BootstrapInfo struct {
	root   *flow.Block
	result *flow.ExecutionResult
	seal   *flow.Seal
}

// NewState initializes a new state backed by a badger database, applying the
// optional configuration parameters.
func BootstrapState(
	metrics module.ComplianceMetrics,
	tracer module.Tracer,
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
	bootstrapInfo BootstrapInfo,
) (*State, error) {

	s := &State{
		metrics:  metrics,
		tracer:   tracer,
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

	err := s.Bootstrap(bootstrapInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to bootstrap with root block: %v, %w", bootstrapInfo.root.ID(), err)
	}

	return s, nil
}

func (s *State) Bootstrap(info BootstrapInfo) error {
	return operation.RetryOnConflict(s.db.Update, func(tx *badger.Txn) error {

		root, result, seal := info.root, info.result, info.seal

		// NEW: bootstrapping from an arbitrary states requires an execution result and block seal
		// as input; we need to verify them against each other and against the root block

		if result.BlockID != root.ID() {
			return fmt.Errorf("root execution result for wrong block (%x != %x)", result.BlockID, root.ID())
		}

		if seal.BlockID != root.ID() {
			return fmt.Errorf("root block seal for wrong block (%x != %x)", seal.BlockID, root.ID())
		}

		if seal.ResultID != result.ID() {
			return fmt.Errorf("root block seal for wrong execution result (%x != %x)", seal.ResultID, result.ID())
		}

		// EPOCHS: If we bootstrap with epochs, we no longer need identities as a payload to the root block; instead, we
		// want to see two system events with all necessary information: one epoch setup and one epoch commit.

		// We should have exactly two service events, one epoch setup and one epoch commit.
		if len(seal.ServiceEvents) != 2 {
			return fmt.Errorf("root block seal must contain two system events (have %d)", len(seal.ServiceEvents))
		}
		setup, valid := seal.ServiceEvents[0].Event.(*flow.EpochSetup)
		if !valid {
			return fmt.Errorf("first service event should be epoch setup (%T)", seal.ServiceEvents[0])
		}
		commit, valid := seal.ServiceEvents[1].Event.(*flow.EpochCommit)
		if !valid {
			return fmt.Errorf("second event should be epoch commit (%T)", seal.ServiceEvents[1])
		}

		// They should both have the same epoch counter to be valid.
		if setup.Counter != commit.Counter {
			return fmt.Errorf("epoch setup counter differs from epoch commit counter (%d != %d)", setup.Counter, commit.Counter)
		}

		// The final view of the epoch must be greater than the view of the first block
		if root.Header.View >= setup.FinalView {
			return fmt.Errorf("final view of epoch less than first block view")
		}

		// They should also both be valid within themselves.
		err := validSetup(setup)
		if err != nil {
			return fmt.Errorf("invalid epoch setup event: %w", err)
		}
		err = validCommit(commit, setup)
		if err != nil {
			return fmt.Errorf("invalid epoch commit event: %w", err)
		}

		// FIRST: validate the root block and its payload

		// NOTE: we might need to relax these restrictions and find a way to process the
		// payload of the root block once we implement epochs

		// the root block should have an empty guarantee payload
		if len(root.Payload.Guarantees) > 0 {
			return fmt.Errorf("root block must not have guarantees")
		}

		// the root block should have an empty seal payload
		if len(root.Payload.Seals) > 0 {
			return fmt.Errorf("root block must not have seals")
		}

		// SECOND: insert the initial protocol state data into the database

		// 1) insert the root block with its payload into the state and index it
		err = s.blocks.StoreTx(root)(tx)
		if err != nil {
			return fmt.Errorf("could not insert root block: %w", err)
		}
		err = operation.InsertBlockValidity(root.ID(), true)(tx)
		if err != nil {
			return fmt.Errorf("could not mark root block as valid: %w", err)
		}

		err = operation.IndexBlockHeight(root.Header.Height, root.ID())(tx)
		if err != nil {
			return fmt.Errorf("could not index root block: %w", err)
		}
		// root block has no parent, so only needs to add one index
		// to indicate the root block has no child yet
		err = operation.InsertBlockChildren(root.ID(), nil)(tx)
		if err != nil {
			return fmt.Errorf("could not initialize root child index: %w", err)
		}

		// 2) insert the root execution result into the database and index it
		err = operation.InsertExecutionResult(result)(tx)
		if err != nil {
			return fmt.Errorf("could not insert root result: %w", err)
		}
		err = operation.IndexExecutionResult(root.ID(), result.ID())(tx)
		if err != nil {
			return fmt.Errorf("could not index root result: %w", err)
		}

		// 3) insert the root block seal into the database and index it
		err = operation.InsertSeal(seal.ID(), seal)(tx)
		if err != nil {
			return fmt.Errorf("could not insert root seal: %w", err)
		}
		err = operation.IndexBlockSeal(root.ID(), seal.ID())(tx)
		if err != nil {
			return fmt.Errorf("could not index root block seal: %w", err)
		}

		// 4) initialize the current protocol state values
		err = operation.InsertStartedView(root.Header.ChainID, root.Header.View)(tx)
		if err != nil {
			return fmt.Errorf("could not insert started view: %w", err)
		}
		err = operation.InsertVotedView(root.Header.ChainID, root.Header.View)(tx)
		if err != nil {
			return fmt.Errorf("could not insert started view: %w", err)
		}
		err = operation.InsertRootHeight(root.Header.Height)(tx)
		if err != nil {
			return fmt.Errorf("could not insert root height: %w", err)
		}
		err = operation.InsertFinalizedHeight(root.Header.Height)(tx)
		if err != nil {
			return fmt.Errorf("could not insert finalized height: %w", err)
		}
		err = operation.InsertSealedHeight(root.Header.Height)(tx)
		if err != nil {
			return fmt.Errorf("could not insert sealed height: %w", err)
		}

		// 5) initialize values related to the epoch logic
		setup.FirstView = root.Header.View // cache the first view of the epoch
		err = s.epoch.setups.StoreTx(setup)(tx)
		if err != nil {
			return fmt.Errorf("could not insert EpochSetup event: %w", err)
		}
		err = s.epoch.commits.StoreTx(commit)(tx)
		if err != nil {
			return fmt.Errorf("could not insert EpochCommit event: %w", err)
		}
		status, err := flow.NewEpochStatus(root.ID(), setup.ID(), commit.ID(), flow.ZeroID, flow.ZeroID)
		if err != nil {
			return fmt.Errorf("could not construct root epoch status: %w", err)
		}
		err = s.epoch.statuses.StoreTx(root.ID(), status)(tx)
		if err != nil {
			return fmt.Errorf("could not insert EpochStatus: %w", err)
		}

		s.metrics.FinalizedHeight(root.Header.Height)
		s.metrics.BlockFinalized(root)

		s.metrics.SealedHeight(root.Header.Height)
		s.metrics.BlockSealed(root)

		return nil
	})
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
