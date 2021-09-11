// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/invalid"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

type State struct {
	metrics module.ComplianceMetrics
	db      *badger.DB
	headers storage.Headers
	blocks  storage.Blocks
	results storage.ExecutionResults
	seals   storage.Seals
	epoch   struct {
		setups   storage.EpochSetups
		commits  storage.EpochCommits
		statuses storage.EpochStatuses
	}
}

type BootstrapConfig struct {
	// SkipNetworkAddressValidation flags allows skipping all the network address related validations not needed for
	// an unstaked node
	SkipNetworkAddressValidation bool
}

func defaultBootstrapConfig() *BootstrapConfig {
	return &BootstrapConfig{
		SkipNetworkAddressValidation: false,
	}
}

type BootstrapConfigOptions func(conf *BootstrapConfig)

func SkipNetworkAddressValidation(conf *BootstrapConfig) {
	conf.SkipNetworkAddressValidation = true
}

func Bootstrap(
	metrics module.ComplianceMetrics,
	db *badger.DB,
	headers storage.Headers,
	seals storage.Seals,
	results storage.ExecutionResults,
	blocks storage.Blocks,
	setups storage.EpochSetups,
	commits storage.EpochCommits,
	statuses storage.EpochStatuses,
	root protocol.Snapshot,
	options ...BootstrapConfigOptions,
) (*State, error) {

	config := defaultBootstrapConfig()
	for _, opt := range options {
		opt(config)
	}

	isBootstrapped, err := IsBootstrapped(db)
	if err != nil {
		return nil, fmt.Errorf("failed to determine whether database contains bootstrapped state: %w", err)
	}
	if isBootstrapped {
		return nil, fmt.Errorf("expected empty database")
	}

	state := newState(metrics, db, headers, seals, results, blocks, setups, commits, statuses)

	if err := isValidRootSnapshot(root, !config.SkipNetworkAddressValidation); err != nil {
		return nil, fmt.Errorf("cannot bootstrap invalid root snapshot: %w", err)
	}

	err = operation.RetryOnConflictTx(db, transaction.Update, func(tx *transaction.Tx) error {

		// 1) insert each block in the root chain segment
		err = state.bootstrapSealingSegment(root)(tx)
		if err != nil {
			return fmt.Errorf("could not bootstrap sealing chain segment: %w", err)
		}

		segment, err := root.SealingSegment()
		if err != nil {
			return fmt.Errorf("could not get sealing segment: %w", err)
		}
		// sealing segment is in ascending height order, so the tail is the
		// oldest ancestor and head is the newest child in the segment
		// TAIL <- ... <- HEAD
		head := segment[len(segment)-1] // reference block of the snapshot
		tail := segment[0]              // last sealed block

		// 2) insert the root execution result and seal into the database and index it
		err = transaction.WithTx(state.bootstrapSealedResult(root))(tx)
		if err != nil {
			return fmt.Errorf("could not bootstrap sealed result: %w", err)
		}

		// 3) insert the root quorum certificate into the database
		qc, err := root.QuorumCertificate()
		if err != nil {
			return fmt.Errorf("could not get root qc: %w", err)
		}
		err = transaction.WithTx(operation.InsertRootQuorumCertificate(qc))(tx)
		if err != nil {
			return fmt.Errorf("could not insert root qc: %w", err)
		}

		// 4) initialize the current protocol state height/view pointers
		err = transaction.WithTx(state.bootstrapStatePointers(root))(tx)
		if err != nil {
			return fmt.Errorf("could not bootstrap height/view pointers: %w", err)
		}

		// 5) initialize values related to the epoch logic
		err = state.bootstrapEpoch(root, !config.SkipNetworkAddressValidation)(tx)
		if err != nil {
			return fmt.Errorf("could not bootstrap epoch values: %w", err)
		}

		// 6) set metric values
		err = state.updateEpochMetrics(root)
		if err != nil {
			return fmt.Errorf("could not update epoch metrics: %w", err)
		}
		state.metrics.BlockSealed(tail)
		state.metrics.SealedHeight(tail.Header.Height)
		state.metrics.FinalizedHeight(head.Header.Height)
		for _, block := range segment {
			state.metrics.BlockFinalized(block)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("bootstrapping failed: %w", err)
	}

	return state, nil
}

// bootstrapSealingSegment inserts all blocks and associated metadata for the
// protocol state root snapshot to disk.
func (state *State) bootstrapSealingSegment(root protocol.Snapshot) func(*transaction.Tx) error {
	return func(tx *transaction.Tx) error {
		segment, err := root.SealingSegment()
		if err != nil {
			return fmt.Errorf("could not get sealing segment: %w", err)
		}
		head := segment[len(segment)-1]

		for i, block := range segment {
			blockID := block.ID()
			height := block.Header.Height

			err := state.blocks.StoreTx(block)(tx)
			if err != nil {
				return fmt.Errorf("could not insert root block: %w", err)
			}
			err = transaction.WithTx(operation.InsertBlockValidity(blockID, true))(tx)
			if err != nil {
				return fmt.Errorf("could not mark root block as valid: %w", err)
			}
			err = transaction.WithTx(operation.IndexBlockHeight(height, blockID))(tx)
			if err != nil {
				return fmt.Errorf("could not index root block segment (id=%x): %w", blockID, err)
			}

			// for all but the first block in the segment, index the parent->child relationship
			if i > 0 {
				err = transaction.WithTx(operation.InsertBlockChildren(block.Header.ParentID, []flow.Identifier{blockID}))(tx)
				if err != nil {
					return fmt.Errorf("could not insert child index for block (id=%x): %w", blockID, err)
				}
			}
		}

		// insert an empty child index for the final block in the segment
		err = transaction.WithTx(operation.InsertBlockChildren(head.ID(), nil))(tx)
		if err != nil {
			return fmt.Errorf("could not insert child index for head block (id=%x): %w", head.ID(), err)
		}

		return nil
	}
}

func (state *State) bootstrapSealedResult(root protocol.Snapshot) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {
		head, err := root.Head()
		if err != nil {
			return fmt.Errorf("could not get head from root snapshot: %w", err)
		}
		result, seal, err := root.SealedResult()
		if err != nil {
			return fmt.Errorf("could not get sealed result from root snapshot: %w", err)
		}

		err = operation.SkipDuplicates(operation.InsertExecutionResult(result))(tx)
		if err != nil {
			return fmt.Errorf("could not insert root result: %w", err)
		}
		err = operation.IndexExecutionResult(result.BlockID, result.ID())(tx)
		if err != nil {
			return fmt.Errorf("could not index root result: %w", err)
		}

		err = operation.SkipDuplicates(operation.InsertSeal(seal.ID(), seal))(tx)
		if err != nil {
			return fmt.Errorf("could not insert root seal: %w", err)
		}
		err = operation.IndexBlockSeal(head.ID(), seal.ID())(tx)
		if err != nil {
			return fmt.Errorf("could not index root block seal: %w", err)
		}

		return nil
	}
}

// bootstrapStatePointers instantiates special pointers used to by the protocol
// state to keep track of special block heights and views.
func (state *State) bootstrapStatePointers(root protocol.Snapshot) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {
		segment, err := root.SealingSegment()
		if err != nil {
			return fmt.Errorf("could not get sealing segment: %w", err)
		}
		head := segment[len(segment)-1]
		tail := segment[0]

		// insert initial views for HotStuff
		err = operation.InsertStartedView(head.Header.ChainID, head.Header.View)(tx)
		if err != nil {
			return fmt.Errorf("could not insert started view: %w", err)
		}
		err = operation.InsertVotedView(head.Header.ChainID, head.Header.View)(tx)
		if err != nil {
			return fmt.Errorf("could not insert started view: %w", err)
		}

		// insert height pointers
		err = operation.InsertRootHeight(head.Header.Height)(tx)
		if err != nil {
			return fmt.Errorf("could not insert root height: %w", err)
		}
		err = operation.InsertFinalizedHeight(head.Header.Height)(tx)
		if err != nil {
			return fmt.Errorf("could not insert finalized height: %w", err)
		}
		err = operation.InsertSealedHeight(tail.Header.Height)(tx)
		if err != nil {
			return fmt.Errorf("could not insert sealed height: %w", err)
		}

		return nil
	}
}

// bootstrapEpoch bootstraps the protocol state database with information about
// the previous, current, and next epochs as of the root snapshot.
//
// The root snapshot's sealing segment must not straddle any epoch transitions
// or epoch phase transitions.
func (state *State) bootstrapEpoch(root protocol.Snapshot, verifyNetworkAddress bool) func(*transaction.Tx) error {
	return func(tx *transaction.Tx) error {
		previous := root.Epochs().Previous()
		current := root.Epochs().Current()
		next := root.Epochs().Next()

		// build the status as we go
		status := new(flow.EpochStatus)
		var setups []*flow.EpochSetup
		var commits []*flow.EpochCommit

		// insert previous epoch if it exists
		_, err := previous.Counter()
		if err == nil {
			// if there is a previous epoch, both setup and commit events must exist
			setup, err := protocol.ToEpochSetup(previous)
			if err != nil {
				return fmt.Errorf("could not get previous epoch setup event: %w", err)
			}
			commit, err := protocol.ToEpochCommit(previous)
			if err != nil {
				return fmt.Errorf("could not get previous epoch commit event: %w", err)
			}

			if err := verifyEpochSetup(setup, verifyNetworkAddress); err != nil {
				return fmt.Errorf("invalid setup: %w", err)
			}

			if err := isValidEpochCommit(commit, setup); err != nil {
				return fmt.Errorf("invalid commit")
			}

			setups = append(setups, setup)
			commits = append(commits, commit)
			status.PreviousEpoch.SetupID = setup.ID()
			status.PreviousEpoch.CommitID = commit.ID()
		} else if !errors.Is(err, protocol.ErrNoPreviousEpoch) {
			return fmt.Errorf("could not retrieve previous epoch: %w", err)
		}

		// insert current epoch - both setup and commit events must exist
		setup, err := protocol.ToEpochSetup(current)
		if err != nil {
			return fmt.Errorf("could not get current epoch setup event: %w", err)
		}
		commit, err := protocol.ToEpochCommit(current)
		if err != nil {
			return fmt.Errorf("could not get current epoch commit event: %w", err)
		}

		if err := verifyEpochSetup(setup, verifyNetworkAddress); err != nil {
			return fmt.Errorf("invalid setup: %w", err)
		}

		if err := isValidEpochCommit(commit, setup); err != nil {
			return fmt.Errorf("invalid commit")
		}

		setups = append(setups, setup)
		commits = append(commits, commit)
		status.CurrentEpoch.SetupID = setup.ID()
		status.CurrentEpoch.CommitID = commit.ID()

		// insert next epoch, if it exists
		_, err = next.Counter()
		if err == nil {
			// either only the setup event, or both the setup and commit events must exist
			setup, err := protocol.ToEpochSetup(next)
			if err != nil {
				return fmt.Errorf("could not get next epoch setup event: %w", err)
			}

			if err := verifyEpochSetup(setup, verifyNetworkAddress); err != nil {
				return fmt.Errorf("invalid setup: %w", err)
			}

			setups = append(setups, setup)
			status.NextEpoch.SetupID = setup.ID()
			commit, err := protocol.ToEpochCommit(next)
			if err != nil && !errors.Is(err, protocol.ErrEpochNotCommitted) {
				return fmt.Errorf("could not get next epoch commit event: %w", err)
			}
			if err == nil {
				if err := isValidEpochCommit(commit, setup); err != nil {
					return fmt.Errorf("invalid commit")
				}
				commits = append(commits, commit)
				status.NextEpoch.CommitID = commit.ID()
			}
		} else if !errors.Is(err, protocol.ErrNextEpochNotSetup) {
			return fmt.Errorf("could not get next epoch: %w", err)
		}

		// sanity check: ensure epoch status is valid
		err = status.Check()
		if err != nil {
			return fmt.Errorf("bootstrapping resulting in invalid epoch status: %w", err)
		}

		// insert all epoch setup/commit service events
		for _, setup := range setups {
			err = state.epoch.setups.StoreTx(setup)(tx)
			if err != nil {
				return fmt.Errorf("could not store epoch setup event: %w", err)
			}
		}
		for _, commit := range commits {
			err = state.epoch.commits.StoreTx(commit)(tx)
			if err != nil {
				return fmt.Errorf("could not store epoch commit event: %w", err)
			}
		}

		// NOTE: as specified in the godoc, this code assumes that each block
		// in the sealing segment in within the same phase within the same epoch.
		segment, err := root.SealingSegment()
		if err != nil {
			return fmt.Errorf("could not get sealing segment: %w", err)
		}
		for _, block := range segment {
			blockID := block.ID()
			err = state.epoch.statuses.StoreTx(blockID, status)(tx)
			if err != nil {
				return fmt.Errorf("could not store epoch status for block (id=%x): %w", blockID, err)
			}
		}

		return nil
	}
}

func OpenState(
	metrics module.ComplianceMetrics,
	db *badger.DB,
	headers storage.Headers,
	seals storage.Seals,
	results storage.ExecutionResults,
	blocks storage.Blocks,
	setups storage.EpochSetups,
	commits storage.EpochCommits,
	statuses storage.EpochStatuses,
) (*State, error) {
	isBootstrapped, err := IsBootstrapped(db)
	if err != nil {
		return nil, fmt.Errorf("failed to determine whether database contains bootstrapped state: %w", err)
	}
	if !isBootstrapped {
		return nil, fmt.Errorf("expected database to contain bootstrapped state")
	}
	state := newState(metrics, db, headers, seals, results, blocks, setups, commits, statuses)

	finalSnapshot := state.Final()

	// update all epoch related metrics
	err = state.updateEpochMetrics(finalSnapshot)
	if err != nil {
		return nil, fmt.Errorf("failed to update epoch metrics: %w", err)
	}

	return state, nil
}

func (s *State) Params() protocol.Params {
	return &Params{state: s}
}

func (s *State) Sealed() protocol.Snapshot {
	// retrieve the latest sealed height
	var sealed uint64
	err := s.db.View(operation.RetrieveSealedHeight(&sealed))
	if err != nil {
		return invalid.NewSnapshot(fmt.Errorf("could not retrieve sealed height: %w", err))
	}
	return s.AtHeight(sealed)
}

func (s *State) Final() protocol.Snapshot {
	// retrieve the latest finalized height
	var finalized uint64
	err := s.db.View(operation.RetrieveFinalizedHeight(&finalized))
	if err != nil {
		return invalid.NewSnapshot(fmt.Errorf("could not retrieve finalized height: %w", err))
	}
	return s.AtHeight(finalized)
}

func (s *State) AtHeight(height uint64) protocol.Snapshot {
	// retrieve the block ID for the finalized height
	var blockID flow.Identifier
	err := s.db.View(operation.LookupBlockHeight(height, &blockID))
	if err != nil {
		return invalid.NewSnapshot(fmt.Errorf("could not look up block by height: %w", err))
	}
	return NewSnapshot(s, blockID)
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
	results storage.ExecutionResults,
	blocks storage.Blocks,
	setups storage.EpochSetups,
	commits storage.EpochCommits,
	statuses storage.EpochStatuses,
) *State {
	return &State{
		metrics: metrics,
		db:      db,
		headers: headers,
		results: results,
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

// updateEpochMetrics update the `consensus_compliance_current_epoch_counter` and the
// `consensus_compliance_current_epoch_phase` metric
func (state *State) updateEpochMetrics(snap protocol.Snapshot) error {

	// update epoch counter
	counter, err := snap.Epochs().Current().Counter()
	if err != nil {
		return fmt.Errorf("could not get current epoch counter: %w", err)
	}
	state.metrics.CurrentEpochCounter(counter)

	// update epoch phase
	phase, err := snap.Phase()
	if err != nil {
		return fmt.Errorf("could not get current epoch counter: %w", err)
	}
	state.metrics.CurrentEpochPhase(phase)

	// update committed epoch final view
	err = state.updateCommittedEpochFinalView(snap)
	if err != nil {
		return fmt.Errorf("could not update committed epoch final view")
	}

	currentEpochFinalView, err := snap.Epochs().Current().FinalView()
	if err != nil {
		return fmt.Errorf("could not update current epoch final view: %w", err)
	}
	state.metrics.CurrentEpochFinalView(currentEpochFinalView)

	dkgPhase1FinalView, dkgPhase2FinalView, dkgPhase3FinalView, err := protocol.DKGPhaseViews(snap.Epochs().Current())
	if err != nil {
		return fmt.Errorf("could not get dkg phase final view: %w", err)
	}

	state.metrics.CurrentDKGPhase1FinalView(dkgPhase1FinalView)
	state.metrics.CurrentDKGPhase2FinalView(dkgPhase2FinalView)
	state.metrics.CurrentDKGPhase3FinalView(dkgPhase3FinalView)

	return nil
}

// updateCommittedEpochFinalView updates the `committed_epoch_final_view` metric
// based on the current epoch phase of the input snapshot. It should be called
// at startup and during transitions between EpochSetup and EpochCommitted phases.
//
// For example, suppose we have epochs N and N+1.
// If we are in epoch N's Staking or Setup Phase, then epoch N's final view should be the value of the metric.
// If we are in epoch N's Committed Phase, then epoch N+1's final view should be the value of the metric.
func (state *State) updateCommittedEpochFinalView(snap protocol.Snapshot) error {

	phase, err := snap.Phase()
	if err != nil {
		return fmt.Errorf("could not get epoch phase: %w", err)
	}

	// update metric based of epoch phase
	switch phase {
	case flow.EpochPhaseStaking, flow.EpochPhaseSetup:

		// if we are in Staking or Setup phase, then set the metric value to the current epoch's final view
		finalView, err := snap.Epochs().Current().FinalView()
		if err != nil {
			return fmt.Errorf("could not get current epoch final view from snapshot: %w", err)
		}
		state.metrics.CommittedEpochFinalView(finalView)
	case flow.EpochPhaseCommitted:

		// if we are in Committed phase, then set the metric value to the next epoch's final view
		finalView, err := snap.Epochs().Next().FinalView()
		if err != nil {
			return fmt.Errorf("could not get next epoch final view from snapshot: %w", err)
		}
		state.metrics.CommittedEpochFinalView(finalView)
	default:
		return fmt.Errorf("invalid phase: %s", phase)
	}

	return nil
}
